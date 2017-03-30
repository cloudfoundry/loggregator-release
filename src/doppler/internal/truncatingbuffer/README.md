# truncatingbuffer

```truncatingbuffer``` is a buffering library used in a number of places in Loggregator to provide "circuit breaking" when encountering 
back pressure from a TCP link to a downstream consumer (Doppler, Metron, etc).

In Jan 2016 the Loggregator team had concerns about the performance of this library and did a charter story to explore. The team 
found that the performance was very high, on the order of 1M dropsonde messages/sec on a Macbook Pro local deployment. Full details are in
the story [here.](https://www.pivotaltracker.com/story/show/110527436)
