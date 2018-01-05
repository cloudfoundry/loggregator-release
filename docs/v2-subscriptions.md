# V2 Subscriptions

The v2 endpoints (provided via the ReverseLogProxy) offer a feature named selectors. Each selector allows a subscription to
request specific data. This enables a subscription to have the Loggregator servers do filtering and avoid unnecessary IO.

### Selectors

There is a unique selector for each Envelope type. Therefore, if a subscription is only interested in Event envelopes, it
could add a selector that sets the `Message` field of the `Selector` with a `EventSelector`.

Loggregator would then only send `Event` envelopes to the subscription.
