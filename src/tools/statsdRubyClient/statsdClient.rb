require 'statsd'
# Set up a global Statsd client for a server on localhost:8125
$statsd = Statsd.new 'localhost', 8125


ARGF.each do |line|
  inputs = line.split(' ')
  if (inputs.length < 2)
    puts "Wrong number of inputs"
    next
  end

  statsdType = inputs[0]
  name = inputs[1]
  case statsdType
    when "increment"
      $statsd.increment name
    when "decrement"
      $statsd.decrement name
    when "count"
      if (inputs.length < 3)
        puts "count needs 3 or more parameters"
        next
      end
      value = inputs[2].to_i
      sampleRate = inputs.length != 4 ? 1 : inputs[3].to_i
      $statsd.count name, value, sampleRate
    when "gauge"
      if (inputs.length != 3)
        puts "gauge needs 2 parameters"
        next
      end
      value = inputs[2].to_i
      $statsd.gauge name, value
    when "timing"
      if (inputs.length != 3)
        puts "timing needs 2 parameters"
        next
      end
      value = inputs[2].to_i
      $statsd.timing name, value
    else
      puts "Unsupported operation: " + statsdType
      next
  end

end

