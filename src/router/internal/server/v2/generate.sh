#!/bin/bash

go get code.cloudfoundry.org/go-pubsub/pubsub-gen/...

pubsub-gen \
--pointer \
--output=$GOPATH/src/code.cloudfoundry.org/loggregator/router/internal/server/v2/envelope_traverser.gen.go \
--package=v2 \
--sub-structs='{"loggregator_v2.Envelope":"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"}' \
--interfaces='{"isEnvelope_Message":["*Envelope_Log","*Envelope_Counter","*Envelope_Gauge","*Envelope_Timer","*Envelope_Event"]}' \
--struct-name=code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2.Envelope \
--imports='{"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2":"loggregator_v2"}' \
--traverser=envelopeTraverser \
--blacklist-fields=Log.Payload,Log.Type,Counter.Delta,Counter.Total,Timer.Start,Timer.Stop,Timer.Name,Envelope.Timestamp,Envelope.InstanceId,Envelope.DeprecatedTags,Envelope.Tags \
--include-pkg-name=true

gofmt -s -w .
