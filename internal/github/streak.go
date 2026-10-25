package github

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Theyka/github-cards/internal/cards"
)

const contributionQuery = `query contributionGraph($login: String!, $from: DateTime!, $to: DateTime!) {
  user(login: $login) {
    createdAt
    contributionsCollection(from: $from, to: $to) {
      contributionYears
      contributionCalendar {
        weeks {
          contributionDays {
            contributionCount
            date
          }
        }
      }
    }
  }
}`

type contributionResponse struct {
	User *struct {
		CreatedAt               string `json:"createdAt"`
		ContributionsCollection struct {
			ContributionYears    []int `json:"contributionYears"`
			ContributionCalendar struct {
				Weeks []struct {
					ContributionDays []struct {
						ContributionCount int    `json:"contributionCount"`
						Date              string `json:"date"`
					} `json:"contributionDays"`
				} `json:"weeks"`
			} `json:"contributionCalendar"`
		} `json:"contributionsCollection"`
	} `json:"user"`
}

// StreakParams are the inputs of FetchStreak.
type StreakParams struct {
	User         string
	StartingYear *int
	ExcludeDays  []string
	Timezone     string
	Mode         string
}

type dayContribution struct {
	date  string
	count int
}

var (
	sanitizeUserRe = regexp.MustCompile(`[^a-zA-Z0-9-]`)
	validShortDays = map[string]bool{
		"Sun": true, "Mon": true, "Tue": true, "Wed": true, "Thu": true, "Fri": true, "Sat": true,
	}
)

func sanitizeUsername(user string) string {
	return sanitizeUserRe.ReplaceAllString(user, "")
}

// normalizeDays mirrors github-readme-streak-stats' normalizeDays: trim,
// capitalize the first letter and keep the first three characters.
func normalizeDays(days []string) []string {
	out := []string{}
	for _, day := range days {
		day = strings.TrimSpace(day)
		if day == "" {
			continue
		}
		runes := []rune(strings.ToLower(day))
		title := strings.ToUpper(string(runes[0])) + string(runes[1:])
		if len([]rune(title)) > 3 {
			title = string([]rune(title)[:3])
		}
		if validShortDays[title] {
			out = append(out, title)
		}
	}
	return out
}

func isExcludedDay(date string, excludedDays []string) bool {
	if len(excludedDays) == 0 {
		return false
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return false
	}
	short := t.Weekday().String()[:3]
	for _, day := range excludedDays {
		if day == short {
			return true
		}
	}
	return false
}

func previousSunday(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, -int(t.Weekday())).Format("2006-01-02")
}

// FetchStreak fetches a user's contribution streak, mirroring
// github-readme-streak-stats' stats.php.
func (c *Client) FetchStreak(ctx context.Context, p StreakParams) (cards.StreakStats, error) {
	user := sanitizeUsername(p.User)
	if user == "" {
		return cards.StreakStats{}, &Error{Message: "GitHub username is required."}
	}

	loc := time.UTC
	if p.Timezone != "" {
		parsed, err := time.LoadLocation(p.Timezone)
		if err != nil {
			return cards.StreakStats{}, &Error{Message: "Invalid timezone."}
		}
		loc = parsed
	}
	now := time.Now().In(loc)
	today := now.Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")
	currentYear := now.Year()

	fetchYear := func(year int) (*contributionResponse, error) {
		vars := map[string]any{
			"login": user,
			"from":  fmt.Sprintf("%d-01-01T00:00:00Z", year),
			"to":    fmt.Sprintf("%d-12-31T23:59:59Z", year),
		}
		data, err := c.GraphQL(ctx, contributionQuery, vars)
		if err != nil {
			return nil, err
		}
		var resp contributionResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, &Error{Message: "Could not parse the GitHub API response.", Kind: KindGraphQL}
		}
		return &resp, nil
	}

	// Fetch the current year first: it carries the account creation date and
	// the list of contribution years.
	current, err := fetchYear(currentYear)
	if err != nil {
		if ghErr, ok := err.(*Error); ok && ghErr.Kind == KindUserNotFound {
			return cards.StreakStats{}, &Error{Message: "Could not find a user with that name."}
		}
		return cards.StreakStats{}, err
	}
	if current.User == nil || current.User.CreatedAt == "" {
		return cards.StreakStats{}, &Error{Message: "Failed to retrieve contributions. This is likely a GitHub API issue."}
	}

	userCreatedYear := 2005
	if len(current.User.CreatedAt) >= 4 {
		if y, convErr := strconv.Atoi(current.User.CreatedAt[:4]); convErr == nil {
			userCreatedYear = y
		}
	}
	minimumYear := userCreatedYear
	if p.StartingYear != nil && *p.StartingYear != 0 {
		minimumYear = *p.StartingYear
	}
	if minimumYear < 2005 {
		minimumYear = 2005
	}

	var yearsToRequest []int
	for y := minimumYear; y <= currentYear-1; y++ {
		yearsToRequest = append(yearsToRequest, y)
	}

	contributionYears := current.User.ContributionsCollection.ContributionYears
	firstContributionYear := userCreatedYear
	if len(contributionYears) > 0 {
		firstContributionYear = contributionYears[len(contributionYears)-1]
	}
	if firstContributionYear < 2005 {
		yearsToRequest = append([]int{firstContributionYear}, yearsToRequest...)
	}

	// Fetch previous years concurrently; failing years are skipped like
	// upstream's executeContributionGraphRequests.
	graphs := map[int]*contributionResponse{currentYear: current}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, year := range yearsToRequest {
		year := year
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resp, fetchErr := fetchYear(year)
			if fetchErr != nil || resp == nil || resp.User == nil {
				return
			}
			mu.Lock()
			graphs[year] = resp
			mu.Unlock()
		}()
	}
	wg.Wait()

	years := make([]int, 0, len(graphs))
	for year := range graphs {
		years = append(years, year)
	}
	sort.Ints(years)

	var contributions []dayContribution
	for _, year := range years {
		u := graphs[year].User
		if u == nil {
			continue
		}
		for _, week := range u.ContributionsCollection.ContributionCalendar.Weeks {
			for _, day := range week.ContributionDays {
				// Count contributions up until today; also count tomorrow if
				// the user contributed already (timezone differences).
				if day.Date <= today || (day.Date == tomorrow && day.ContributionCount > 0) {
					contributions = append(contributions, dayContribution{date: day.Date, count: day.ContributionCount})
				}
			}
		}
	}
	if len(contributions) == 0 {
		return cards.StreakStats{}, &Error{Message: "No contributions found."}
	}
	sort.SliceStable(contributions, func(i, j int) bool { return contributions[i].date < contributions[j].date })

	var stats cards.StreakStats
	if strings.ToLower(p.Mode) == "weekly" {
		stats = computeWeeklyStats(contributions)
	} else {
		stats = computeDailyStats(contributions, normalizeDays(p.ExcludeDays))
	}
	return stats, nil
}

// computeDailyStats mirrors getContributionStats.
func computeDailyStats(contributions []dayContribution, excludedDays []string) cards.StreakStats {
	lastDate := contributions[len(contributions)-1].date
	firstDate := contributions[0].date
	stats := cards.StreakStats{
		Mode:              "daily",
		FirstContribution: "",
		LongestStreak:     cards.StreakLength{Start: firstDate, End: firstDate, Length: 0},
		CurrentStreak:     cards.StreakLength{Start: firstDate, End: firstDate, Length: 0},
		ExcludedDays:      excludedDays,
	}

	for _, dc := range contributions {
		stats.TotalContributions += float64(dc.count)
		excluded := isExcludedDay(dc.date, excludedDays)
		if dc.count > 0 || (stats.CurrentStreak.Length > 0 && excluded) {
			stats.CurrentStreak.Length++
			stats.CurrentStreak.End = dc.date
			if stats.CurrentStreak.Length == 1 {
				stats.CurrentStreak.Start = dc.date
			}
			if stats.FirstContribution == "" {
				stats.FirstContribution = dc.date
			}
			if stats.CurrentStreak.Length > stats.LongestStreak.Length {
				stats.LongestStreak = stats.CurrentStreak
			}
		} else if dc.date != lastDate {
			stats.CurrentStreak.Length = 0
			stats.CurrentStreak.Start = lastDate
			stats.CurrentStreak.End = lastDate
		}
	}
	return stats
}

// computeWeeklyStats mirrors getWeeklyContributionStats.
func computeWeeklyStats(contributions []dayContribution) cards.StreakStats {
	lastDate := contributions[len(contributions)-1].date
	firstDate := contributions[0].date
	thisWeek := previousSunday(lastDate)
	firstWeek := previousSunday(firstDate)

	stats := cards.StreakStats{
		Mode:              "weekly",
		FirstContribution: "",
		LongestStreak:     cards.StreakLength{Start: firstWeek, End: firstWeek, Length: 0},
		CurrentStreak:     cards.StreakLength{Start: firstWeek, End: firstWeek, Length: 0},
	}

	var weekOrder []string
	weekTotals := map[string]int{}
	for _, dc := range contributions {
		week := previousSunday(dc.date)
		if _, seen := weekTotals[week]; !seen {
			weekOrder = append(weekOrder, week)
			weekTotals[week] = 0
		}
		if dc.count > 0 {
			weekTotals[week] += dc.count
			if stats.FirstContribution == "" {
				stats.FirstContribution = dc.date
			}
		}
	}

	for _, week := range weekOrder {
		count := weekTotals[week]
		stats.TotalContributions += float64(count)
		if count > 0 {
			stats.CurrentStreak.Length++
			stats.CurrentStreak.End = week
			if stats.CurrentStreak.Length == 1 {
				stats.CurrentStreak.Start = week
			}
			if stats.CurrentStreak.Length > stats.LongestStreak.Length {
				stats.LongestStreak = stats.CurrentStreak
			}
		} else if week != thisWeek {
			stats.CurrentStreak.Length = 0
			stats.CurrentStreak.Start = thisWeek
			stats.CurrentStreak.End = thisWeek
		}
	}
	return stats
}
