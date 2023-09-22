# Batching
[![GoDoc][go-doc-badge]][go-doc]

If you have any questions, or want to get attention for a PR or issue please reach out on the [#logging-and-metrics channel in the cloudfoundry slack](https://cloudfoundry.slack.com/archives/CUW93AF3M)

`batching` implements a generic `Batcher`. This batcher uses `interface{}` and
should not be used directly. It should be specialized by creating a type that
embeds `Batcher` but accepts concrete types. See `ByteBatcher` for an example
of specializing the `Batcher`. Also, see `example_test.go` for an example of
how to use a specialized batcher.

[go-doc-badge]:             https://godoc.org/code.cloudfoundry.org/go-batching?status.svg
[go-doc]:                   https://godoc.org/code.cloudfoundry.org/go-batching
