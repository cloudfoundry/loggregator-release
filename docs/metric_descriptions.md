## Metrics emitted by the Loggregator subsystem

Loggregator emits a series of metrics as a pulse for healthy operations. **Please note** The names of these metrics will change as new features are released. 

Metric Name as Emitted|Component|Origin|Description|Emitted Consistently or Only on Event?|Notes - Why should someone care about this metric? Why is it valuable?
DopplerServer.messageRouter.numberOfDumpSinks|Loggregator|Doppler|Number of app loop buffers|Consistently|The number of dump sinks has a memory impact on doppler
DopplerServer.listeners.receivedEnvelopes|Loggregator |Doppler|Can be used in conjunction w/MetronAgent.dropsondeMarshaller.sentEnvelopes to calculate specific loss between Metron->Doppler|Consistently|Good for isolating loss and determing if scale will help with log loss. 
DopplerServer.TruncatingBuffer.totalDroppedMessages|Loggregator|Doppler|Total dropped messages by doppler for application syslog drains|Event|Singifies dopplers are not processing messages fast enough
MetronAgent.dropsondeMarshaller.sentEnvelopes|Loggregator|Metron|Can be used in conjunction w/DopplerServer.listeners.receivedEnvelopes to calculate specific loss between Metron->Doppler|Consistently|Good for isolating loss and determing if scale will help with log loss. 
MetronAgent.dropsondeUnmarshaller.receivedEnvelopes|Loggregator|Metron|||
LoggregatorTrafficController.LinuxFileDescriptor|Loggregator|Traffic Controller|Number of connections maintained by Traffic Controller||
LoggregatorTrafficController.listeners.receivedEnvelopes|Loggregator|Traffic Controller|||
