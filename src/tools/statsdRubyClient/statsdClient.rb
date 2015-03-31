require 'statsd'
# Set up a global Statsd client for a server on localhost:8125
$statsd = Statsd.new('localhost', 8125).tap{|sd| sd.namespace = 'testNamespace'}

ARGF.each do |line|
  inputs = line.split(' ')
  if (inputs.length < 3)
    puts "Wrong number of inputs, 3 needed at least"
    next
  end

  statsd_type = inputs[0]
  name = inputs[1]
  value = inputs[2].to_i
  case statsd_type
    when "count"
      sample_rate = inputs.length != 4 ? 1 : inputs[3].to_f
      $statsd.count name, value, sample_rate
    when "gauge"
      $statsd.gauge name, value
    when "timing"
      $statsd.timing name, value
    else
      puts "Unsupported operation: " + statsd_type
      next
  end

end

