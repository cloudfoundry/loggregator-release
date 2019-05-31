# Batching
[![GoDoc][go-doc-badge]][go-doc] [![travis][travis-badge]][travis]

`batching` implements a generic `Batcher`. This batcher uses `interface{}` and
should not be used directly. It should be specialized by creating a type that
embeds `Batcher` but accepts concrete types. See `ByteBatcher` for an example
of specializing the `Batcher`. Also, see `example_test.go` for an example of
how to use a specialized batcher.

[go-doc-badge]:             https://godoc.org/code.cloudfoundry.org/go-batching?status.svg
[go-doc]:                   https://godoc.org/code.cloudfoundry.org/go-batching
[travis-badge]:             https://travis-ci.org/cloudfoundry/go-batching.svg?branch=master
[travis]:                   https://travis-ci.org/cloudfoundry/go-batching?branch=master
