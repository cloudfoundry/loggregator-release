#!/bin/bash

go get code.cloudfoundry.org/go-pubsub/pubsub-gen/...

pubsub-gen \
--pointer \
--output=$GOPATH/src/code.cloudfoundry.org/loggregator/doppler/internal/server/v2/envelope_traverser.gen.go \
--package=v2 \
--sub-structs='{"loggregator_v2.Envelope":"code.cloudfoundry.org/loggregator/plumbing/v2"}' \
--interfaces='{"isEnvelope_Message":["*Envelope_Log","*Envelope_Counter","*Envelope_Gauge","*Envelope_Timer","*Envelope_Event"]}' \
--struct-name=code.cloudfoundry.org/loggregator/plumbing/v2.Envelope \
--imports='{"code.cloudfoundry.org/loggregator/plumbing/v2":"v2"}' \
--traverser=envelopeTraverser \
--blacklist-fields=Log.Payload,Log.Type,Counter.Name,Counter.Delta,Counter.Total,Gauge.Metrics,Timer.Start,Timer.Stop,Timer.Name,Envelope.Timestamp,Envelope.InstanceId,Envelope.DeprecatedTags,Envelope.Tags \
--include-pkg-name=true
