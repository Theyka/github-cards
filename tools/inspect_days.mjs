// Debug helper: prints the raw contribution calendar for a user/year.
// Usage: $env:GH_TOKEN = gh auth token; node tools/inspect_days.mjs Theyka 2026
const [, , login = "Theyka", yearArg = "2026"] = process.argv;
const token = process.env.GH_TOKEN;
if (!token) {
  console.error("GH_TOKEN not set");
  process.exit(1);
}
const year = parseInt(yearArg, 10);

const query = `query($login:String!, $from:DateTime!, $to:DateTime!){
  user(login:$login){
    contributionsCollection(from:$from, to:$to){
      contributionCalendar { weeks { contributionDays { date contributionCount } } }
    }
  }
}`;

const res = await fetch("https://api.github.com/graphql", {
  method: "POST",
  headers: {
    Authorization: `bearer ${token}`,
    "Content-Type": "application/json",
    "User-Agent": "github-cards-debug",
  },
  body: JSON.stringify({
    query,
    variables: {
      login,
      from: `${year}-01-01T00:00:00Z`,
      to: `${year}-12-31T23:59:59Z`,
    },
  }),
});
const json = await res.json();
if (json.errors) {
  console.error(JSON.stringify(json.errors, null, 2));
  process.exit(1);
}
const days = json.data.user.contributionsCollection.contributionCalendar.weeks.flatMap(
  (w) => w.contributionDays,
);

console.log(`total days: ${days.length}, first ${days[0].date}, last ${days[days.length - 1].date}`);
console.log(`total contributions: ${days.reduce((a, d) => a + d.contributionCount, 0)}`);

let cur = 0,
  longest = 0,
  start = null,
  lStart = null,
  lEnd = null;
for (const d of days) {
  if (d.contributionCount > 0) {
    if (cur === 0) start = d.date;
    cur++;
    if (cur > longest) {
      longest = cur;
      lStart = start;
      lEnd = d.date;
    }
  } else {
    cur = 0;
  }
}
console.log(`longest in ${year}: ${longest} (${lStart} - ${lEnd})`);

console.log("positive days in April and August:");
for (const d of days) {
  if ((d.date.startsWith(`${year}-04`) || d.date.startsWith(`${year}-08`)) && d.contributionCount > 0) {
    console.log(`  ${d.date} -> ${d.contributionCount}`);
  }
}
