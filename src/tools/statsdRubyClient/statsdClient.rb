require 'statsd'
# Set up a global Statsd client for a server on localhost:8125
$statsd = Statsd.new 'localhost', 8125


ARGF.each do |line|
  inputs = line.split(' ')
  if (inputs.length < 3)
    puts "Wrong number of inputs, 3 needed at least"
    next
  end

  statsdType = inputs[0]
  name = inputs[1]
  value = inputs[2].to_i
  case statsdType
    when "count"
      sampleRate = inputs.length != 4 ? 1 : inputs[3].to_f
      $statsd.count name, value, sampleRate
    when "gauge"
      $statsd.gauge name, value
    when "timing"
      $statsd.timing name, value
    else
      puts "Unsupported operation: " + statsdType
      next
  end

end

