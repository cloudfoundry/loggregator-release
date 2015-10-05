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
    route_registrar:
      routes:
      - name: doppler
        port: 8081
        uris:
        - doppler.#{system_domain}
      - name: loggregator
        port: 8080
        uris:
        - loggregator.#{system_domain}
    EOF
    result.chomp
  end

  def loggregator_trafficcontroller_z2_properties
    result = <<-EOF
    metron_agent:
      zone: z2
    traffic_controller:
      zone: z2
    route_registrar:
      routes:
      - name: doppler
        port: 8081
        uris:
        - doppler.#{system_domain}
      - name: loggregator
        port: 8080
        uris:
        - loggregator.#{system_domain}
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
    - name: route_registrar
      release: cf
    EOF
    result.chomp
  end

  def aws_lamb_properties(deployment_name)
    result = <<-EOF
  loggregator:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges:
    - start: 10.10.0.0
      end: 10.10.255.255
    outgoing_dropsonde_port: 8081
    etcd:
      machines:
      - 0.0.0.10
      - 0.0.0.25
      - 0.0.1.9

  doppler:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges: null
    unmarshaller_count: 5
    port: 4443

  metron_agent:
    deployment: #{deployment_name}

  traffic_controller:
    outgoing_port: 8080
    EOF
    result.chomp
  end

  def lamb_properties(deployment_name)
    return aws_lamb_properties(deployment_name) if @infrastructure == 'aws'

    etcd_machines = {
      'vsphere' => "['0.0.0.14', '0.0.0.15', '0.0.1.13']",
      'openstack' => "['0.0.0.8']",
      'warden' => "['10.244.0.42']",
    }

    result = <<-EOF
  loggregator:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges: null
    outgoing_dropsonde_port: 8081
    etcd:
      machines: #{etcd_machines[@infrastructure]}

  doppler:
    maxRetainedLogMessages: 100
    debug: false
    blacklisted_syslog_ranges: null
    unmarshaller_count: 5
    port: 4443

  metron_agent:
    deployment: #{deployment_name}

  traffic_controller:
    outgoing_port: 8080
    EOF
    result.chomp
  end

  def system_domain
    case @infrastructure
      when "vsphere"
        "0.0.0.3.xip.io"
      when "warden"
        "bosh-lite.com"
      else
        "example.com"
    end
  end

  def get_binding
    binding
  end
end
