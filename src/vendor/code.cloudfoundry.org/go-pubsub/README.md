
![pubsub][pubsub-logo]

[![GoDoc][go-doc-badge]][go-doc] [![travis][travis-badge]][travis]

PubSub publishes data to subscriptions. However, it can do so much more than
just push some data to a subscription.  Each subscription is placed in a tree.
When data is published, it traverses the tree and finds each interested
subscription. This allows for sophisticated filters and routing.

### Installation

```bash
go get code.cloudfoundry.org/go-pubsub
```

### Subscription Trees

A subscription tree is a collection of subscriptions that are organized based
on what data they want published to     them. When subscribing, a path is
provided to give directions to PubSub about where to store the subscription
and what data should be published to it.

So for example, say there are three subscriptions with the following paths:

| Name  | Path              |
|-------|-------------------|
| sub-1 | `["a", "b", "c"]` |
| sub-2 | `["a", "b", "d"]` |
| sub-3 | `["a", "b"]`      |

After both subscriptions have been registered PubSub will have the following
subscription tree:
```
              a
              |
              b   <-(sub-3)
             / \
(sub-1)->   c   d   <-(sub-2)
```

To better draw out each's subscriptions view of the tree:

##### Sub-1

```
              a
              |
              b
             /
(sub-1)->   c
```

##### Sub-2

```
              a
              |
              b
               \
                d   <-(sub-2)
```

##### Sub-3

```
              a
              |
              b   <-(sub-3)
```

So to give a few exapmles of how data could be published:

###### Single path

```
              a
              |
              b   <-(sub-3)
               \
                d   <-(sub-2)
```

In this example both `sub-2` and `sub-3` would have the data written to it.

###### Multi-Path

```
              a
              |
              b   <-(sub-3)
             / \
(sub-1)->   c   d   <-(sub-2)
```

In this example all `sub-1`, `sub-2` and `sub-3` would have the data written
to it.

##### Shorter Path

```
              a
              |
              b   <-(sub-3)
```

In this example only `sub-3` would have the data written to it.

##### Other Path

```
              x
              |
              y
```

In this example, no subscriptions would have data written to them.

### Simple Example:

```go
ps := pubsub.New()
subscription := func(name string) pubsub.SubscriptionFunc {
	return func(data interface{}) {
		fmt.Printf("%s -> %v\n", name, data)
	}
}

ps.Subscribe(subscription("sub-1"), []string{"a", "b", "c"})
ps.Subscribe(subscription("sub-2"), []string{"a", "b", "d"})
ps.Subscribe(subscription("sub-3"), []string{"a", "b", "e"})

ps.Publish("data-1", pubsub.LinearTreeTraverser([]string{"a", "b"}))
ps.Publish("data-2", pubsub.LinearTreeTraverser([]string{"a", "b", "c"}))
ps.Publish("data-3", pubsub.LinearTreeTraverser([]string{"a", "b", "d"}))
ps.Publish("data-3", pubsub.LinearTreeTraverser([]string{"x", "y"}))

// Output:
// sub-1 -> data-2
// sub-2 -> data-3
```

In this example the `LinearTreeTraverser` is used to traverse the tree of
subscriptions. When an interested subscription is found (in this case `sub-1`
and `sub-2` for `data-2` and `data-3` respectively), the subscription is
handed the data.

More complex examples can be found in the
[examples](https://code.cloudfoundry.org/go-pubsub/tree/master/examples)
directory.

### TreeTraversers

A `TreeTraverser` is used to traverse the subscription tree and find what
subscriptions should have the data published to them. There are a few
implementations provided, however it is likely a user will need to implement
their own to suit their data.

When creating a `TreeTraverser` it is important to note how the data is
structured. A `TreeTraverser` must be deterministic and ideally stateless. The
order the data is parsed and returned (via `Traverse()`) must align with the
given path of `Subscribe()`.

This means if the `TreeTraverser` intends to look at field A, then B, and then
finally C, then the subscription path must be A, B and then C (and not B, A, C
or something).

### Subscriptions

A `Subscription` is used when publishing data. The given path is used to
determine it's placement in the subscription tree.

### Code Generation

The tree traversers and subscriptions are quite complicated. Laying out a tree
structure is not something humans are going to find natural. Therefore a
[generator](https://github.com/cloudfoundry-incubator/go-pubsub/tree/master/pubsub-gen)
is provided for structs.

The struct is inspected (at `go generate` time) and creates the tree layout
code. There is a provided
[example](https://github.com/cloudfoundry-incubator/go-pubsub/tree/master/examples/structs).

[pubsub-logo]:  https://raw.githubusercontent.com/cloudfoundry/go-pubsub/gh-pages/pubsub-logo.png
[go-doc-badge]: https://godoc.org/code.cloudfoundry.org/go-pubsub?status.svg
[go-doc]:       https://godoc.org/code.cloudfoundry.org/go-pubsub
[travis-badge]: https://travis-ci.org/cloudfoundry/go-pubsub.svg?branch=master
[travis]:       https://travis-ci.org/cloudfoundry/go-pubsub?branch=master
