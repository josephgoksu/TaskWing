# Agent login methods

Agent CLIs log in in many different ways. TaskWing must handle all of them without a real secret entering a box (`PRINCIPLES.md` #3). Facts below come from the boat.dev docs, read 2026-11-12.

## The methods

| Method | Example | Where it lives in a box today | Can a proxy swap it? |
|---|---|---|---|
| API key env var | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, OpenRouter, DeepSeek, Mistral, xAI | Environment variable | Yes, in the `Authorization` or `x-api-key` header |
| Subscription OAuth token | `CLAUDE_CODE_OAUTH_TOKEN`, `CHATGPT_ACCOUNT_ID` | Environment variable | Yes (boat does it, and renews the login outside the box) |
| Subscription login file | `~/.claude/.credentials.json`, `~/.codex/auth.json` | File in the home dir | Yes, if the agent honours a base URL override |
| Custom API | `NAME` + `NAME_BASE_URL` | Environment variable | Only if code calls the base URL; the secret may appear in any header or the query string |
| Request signing | AWS Bedrock (SigV4) | AWS keys | No: nothing to swap |
| Some sign-ins | Kimi Code sign-in | Login file | No (boat withholds it; use an API key instead) |

## How boat's proxy works (the pattern to copy)

```
box: ANTHROPIC_API_KEY=<placeholder>  ANTHROPIC_BASE_URL=<proxy>
  → proxy checks the placeholder, puts the real key in the auth header
  → provider
```

| Fact | Consequence |
|---|---|
| A placeholder is useless outside the proxy and is new at every start | A leaked placeholder is harmless after the box stops |
| Only `Authorization` / `x-api-key` headers are swapped | A placeholder anywhere else stays a placeholder |
| A login that can't be proxied is withheld while the proxy is on | Matches TaskWing's "refuse unless the run opts in" |
| No automatic spend cap; usage is visible per box | TaskWing should track usage per box and block a runaway box |
| Costs $1/day + $1 per 1,000 requests on top of the provider bill | A self-hosted proxy over SSH avoids this but becomes ours to run |

## Claude subscription: what is allowed (read 2026-11-14)

Joseph pays for a flat Claude subscription and does not want to pay per token through the API. These rules decide how TaskWing may use that subscription.

| Setup | Subscription works? | Why |
|---|---|---|
| TaskWing starts the real, unmodified Claude Code and the user signs in through Anthropic's own flow | Yes | The legal page says the rules do not stop a user signing in to the unmodified binary, including when a platform hosts it in sandboxes |
| `claude -p` or the Agent SDK, signed in with the plan | Yes today | Still draws from subscription limits (support article, Oct 7 2026 update). A change was announced, then paused on June 15 2026, so this can move again |
| TaskWing offers its own "sign in with Claude", or calls Anthropic directly with the user's token | No | Third parties may not offer Claude.ai login or route requests through plan credentials |
| A proxy that holds the subscription token and swaps it in | Looks not allowed | Developers "may not collect, store, or intermediate" Claude.ai credentials or session tokens. This is `PRINCIPLES.md` #3 for subscription users, and boat's proxy pattern |
| Another agent (not Claude Code) on the Claude subscription | No | Same rule |

What this means for TaskWing:

- The lead agent is the real Claude Code binary, started on the user's machine. TaskWing never sees or stores the login. No API key needed.
- Drive the `claude` binary itself. Building on the Agent SDK is the path Anthropic points product developers to API keys for.
- Workers in boxes: decided 2026-11-14. The sandbox provider handles the subscription login (boat does, and Joseph has run his Claude subscription through it without a problem). TaskWing never holds it. TaskWing's own proxy is for API keys only. On a provider with no login handling, a subscription is not offered; use an API key.
- Known risk, accepted: a provider proxy relaying a subscription login may itself be against Anthropic's terms. That risk sits with the provider and the account, not with TaskWing's code.
- Plan limits assume "ordinary, individual usage". Nine boxes on one plan may hit limits fast.
- This is a reading of the terms, not legal advice. The page says to contact Anthropic sales for questions on a specific use.

Sources: https://code.claude.com/docs/en/legal-and-compliance and https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan

## Open risks

| Risk | Why it matters |
|---|---|
| Subscription terms | Proxying a Claude or ChatGPT subscription login through our own proxy may break the provider's terms. Check each provider before shipping this path. Google has suspended Antigravity accounts over wrappers around its login token. |
| Base URL support per agent | The proxy only works if the agent CLI honours a base URL override. Unverified per agent. |

## Sources

- [Access and credentials](https://docs.boat.dev/environments/access.md)
- [AI credentials and the proxy](https://docs.boat.dev/ai-credentials.md)
- [Keys and subscriptions](https://docs.boat.dev/integrated-agents/keys.md)
- `boat ai --help`, `boat ai proxy --help` (CLI, 2026-11-12)
