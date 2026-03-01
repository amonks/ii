@specs/README.md

- keep the specifications up to date as you make changes
- always use red-green tdd; we don't have good test coverage at the moment but we would like to. taking time to write tests is good.
- never run 'go build' -- it pollutes the working directory with binary artifacts. use 'go test', which implies 'go build' but goes to tmp
