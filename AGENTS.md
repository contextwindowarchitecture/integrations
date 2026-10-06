# AGENTS.md

Instructions for coding agents and contributors working in `integrations`, where the Context Window Architecture (CWA) draft is integrated into systems that already exist. An integration shows what a real host system would change to assemble its model context with CWA. It is not a tutorial, which is [examples](https://github.com/contextwindowarchitecture/examples)' job, and not a demonstration of CWA's internals, which is [assembler-demo](https://github.com/contextwindowarchitecture/assembler-demo)'s.

## Layout

Each integration is a top-level folder named after its host system (`fullsend/`), self-contained, in the host's language, with its own README, suite, registry, fixtures and committed scenarios. The root README's table lists every integration; add a row when you add one.

## Commands

In a Go integration:

```sh
go test ./...                    # the suite; must pass before every commit
go run ./cmd/scenarios -check    # fail when a committed scenario differs from what the code builds now
go run ./cmd/scenarios -write    # rebuild the committed scenarios after an intended change, then review the diff
gofmt -l .                       # must print nothing
```

An integration in another language documents its equivalents in its README, and CI gets a job for it.

## Rules

- **Read the host before designing.** Find where the host decides what its model sees, and cite it, pinned to a commit, in the README. Design around the seams the host already has. An integration that needs the host to rebuild its model requests should say so as a cost.
- **Depend on a released assembler, never a sibling checkout.** Pin the assembler to a release tag in the language's manifest, with the lock committed (`go.mod` and `go.sum`; `pyproject.toml` and `uv.lock`). Integrations in the same language pin the same release. Move a pin in its own `build:` commit, with the regenerated scenarios and their diff explained in the body.
- **Never assemble in integration code.** Admission, conflict resolution, fitting and rendering belong to the assembler. An integration produces items, freezes a snapshot, calls the assembler and hands over what comes back. If an integration needs behavior the assembler lacks, the spec changes first, in the [specification repository](https://github.com/contextwindowarchitecture/contextwindowarchitecture).
- **Never patch an assembler's output.** A handoff may map the payload onto the host's seams, such as joining system entries into one agent definition, but it adds only text the README names and records each part's hash. A refused assembly has no payload and never reaches a model.
- **Authority comes from the host's own access control, never from content.** Say which host permission decides which slot, and test it.
- **Committed scenarios are reviewed expectations.** Read the diff of every file a `-write` changes before committing, and say in the commit body why the context changed.
- **Say what is unreasonable.** Each README has a section on what CWA should not be used for in that host, and why.
- **Fixtures are fictional.** No real people, credentials, internal hostnames or private data, even when the host is a real project.
- **No model calls in the suite.** Nothing in a suite needs a key or the network beyond fetching dependencies.

## Test-driven development

Behavior changes follow red → green → refactor. Before claiming a test protects something, break the code on purpose and watch the test fail, then restore the file by copying it back, never with `git checkout` or `git restore`.

## Commits

- **Conventional Commits 1.0.0, signed off.** `git commit -s` with `type(scope): summary` in the imperative mood, lower case, no trailing period, at most 72 characters. Types: `feat`, `fix`, `test`, `refactor`, `docs`, `build`, `ci`, `chore`. Scopes: the integration's folder name (`fullsend`), or `repo` and `ci` for shared files. Don't add `Co-Authored-By` trailers.
- **One behavior, or one refactor, per commit.** Every commit passes the suite of every integration it touches.
- **Never push, and never tag.** The remote is `origin` (https://github.com/contextwindowarchitecture/integrations). The maintainer publishes commits.
- Stage paths explicitly. Never commit `.env`, build output or anything under `.claude/`.

## Documentation

A commit that changes behavior also updates the README that describes it. Use Mermaid diagrams wherever a flow reads faster as a picture, and check that each one parses before committing.
