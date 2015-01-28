class LambProperties
  def initialize(infrastructure)
    @infrastructure = infrastructure
  end

  def loggregator_z1_properties
    result = <<-EOF
    metron_agent:
      zone: z1
    doppler:
      zone: z1
    EOF
    result.chomp
  end

  def loggregator_z2_properties
    result = <<-EOF
    metron_agent:
      zone: z2
    doppler:
      zone: z2
    EOF
    result.chomp
  end

  def loggregator_trafficcontroller_z1_properties
    result = <<-EOF
    metron_agent:
      zone: z1
    traffic_controller:
      zone: z1
    EOF
    result.chomp
  end

  def loggregator_trafficcontroller_z2_properties
    result = <<-EOF
    metron_agent:
      zone: z2
    traffic_controller:
      zone: z2
    EOF
    result.chomp
  end

  def loggregator_templates
    result = <<-EOF
    - name: doppler
      release: cf
    - name: syslog_drain_binder
      release: cf
    - name: metron_agent
      release: cf
    EOF
    result.chomp
  end

  def loggregator_trafficcontroller_templates
    result = <<-EOF
    - name: loggregator_trafficcontroller
      release: cf
    - name: metron_agent
      release: cf
    EOF
    result.chomp
  end

  def aws_lamb_properties
    result = <<-EOF
  loggregator:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges:
    - start: 10.10.0.0
      end: 10.10.255.255

  doppler:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges: null
    EOF
    result.chomp
  end

  def lamb_properties
    return aws_lamb_properties if @infrastructure == 'aws'

    result = <<-EOF
  loggregator:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges: null

  doppler:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges: null
    EOF
    result.chomp
  end

  def get_binding
    binding
  end
end
