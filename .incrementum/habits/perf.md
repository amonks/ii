# Performance Audit

Use Go's wonderful profiling tooling to explore and improve the performance
of our system here.

## Workflow

1. Pick an operation (like `ii todo list`)
2. Benchmark it, running with abundant generated test data
3. Profile
4. Make an improvement
5. Benchmark again to show improvement
6. Include both benchmarks in your commit message

Don't be afraid to build tooling for benchmarking, generating test data, or
whatever else.

Keep `./specs/perf.md` updated with current understanding, a log of
improvements made, measurements, tool-use instructions, whatever's useful.
