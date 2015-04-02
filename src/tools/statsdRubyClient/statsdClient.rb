#!/usr/bin/env ruby

require 'statsd'
# Set up a global Statsd client for a server on specified port (or 8125 by default)
port = ARGV[0] || 8125
$statsd = Statsd.new('localhost', port.to_i).tap{|sd| sd.namespace = 'testNamespace'}

while line = $stdin.gets
  inputs = line.split(' ')
  if (inputs.length < 3)
    puts "Wrong number of inputs, 3 needed at least"
    next
  end

  statsd_type = inputs[0]
  name = inputs[1]
  value = inputs[2].to_i
  sample_rate = inputs.length != 4 ? 1 : inputs[3].to_f
  case statsd_type
    when "count"
      $statsd.count name, value, sample_rate
    when "gauge"
      $statsd.gauge name, value, sample_rate
    when "timing"
      $statsd.timing name, value, sample_rate
    else
      puts "Unsupported operation: " + statsd_type
      next
  end

end

