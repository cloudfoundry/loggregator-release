require "json"
require "yaml"
require "bosh/template/test"

include Bosh::Template::Test

RSpec.describe "Doppler JSON" do
  it "renders a complete JSON configuration" do
    properties = {
      "doppler" => {
        "container_metric_ttl_seconds" => 60,
        "disable_announce" => true,
        "grpc_port" => 1111,
        "health_addr" => "localhost:3333",
        "maxRetainedLogMessages" => 100,
        "message_drain_buffer_size" => 20,
        "pprof_port" => 2222,
        "sink_inactivity_timeout_seconds" => 10,
        "unmarshaller_count" => 30,
      },
      "metron_endpoint" => {
        "host" => "10.0.0.10",
        "dropsonde_port" => 4444,
        "grpc_port" => 5555
      },
      "loggregator" => {
        "tls" => {
          "cipher_suites" => "a:b",
        }
      }
    }
    spec = InstanceSpec.new(
      id: "some-id",
      name: "some-job",
      az: "some-az",
      ip: "10.0.0.1",
    )
    config = render_template(properties, spec: spec)

    expected_config = {
      "MessageDrainBufferSize" => 20,
      "GRPC" => {
        "Port" => 1111,
        "KeyFile" => "/var/vcap/jobs/doppler/config/certs/doppler.key",
        "CertFile" => "/var/vcap/jobs/doppler/config/certs/doppler.crt",
        "CAFile" => "/var/vcap/jobs/doppler/config/certs/loggregator_ca.crt",
        "CipherSuites" => ["a", "b"]
      },
      "Zone" => "some-az",
      "IP" => "10.0.0.1",
      "JobName" => "some-job",
      "Index" => "some-id",
      "MaxRetainedLogMessages" => 100,
      "ContainerMetricTTLSeconds" => 60,
      "SinkInactivityTimeoutSeconds" => 10,
      "UnmarshallerCount" => 30,
      "PPROFPort" => 2222,
      "HealthAddr" => "localhost:3333",
      "MetronConfig" => {
        "UDPAddress" => "10.0.0.10:4444",
        "GRPCAddress" => "10.0.0.10:5555"
      },
    }
    expect(config).to eq(expected_config)
  end

  it "includes the IP address" do
    spec = InstanceSpec.new(ip: "1.2.3.4")
    config = render_template({}, spec: spec)

    expect(config["IP"]).to eq("1.2.3.4")
  end

  it "defaults to the job's name" do
    spec = InstanceSpec.new(name: "some-name")
    config = render_template({}, spec: spec)

    expect(config["JobName"]).to eq("some-name")
  end

  describe "Index" do
    it "defaults to the spec's id" do
      spec = InstanceSpec.new(id: "some-id")
      config = render_template({}, spec: spec)

      expect(config["Index"]).to eq("some-id")
    end

    it "fails over to index if id is not present" do
      spec = InstanceSpec.new(index: "some-index", id: nil)
      config = render_template({}, spec: spec)

      expect(config["Index"]). to eq("some-index")
    end
  end

  describe "Zone" do
    it "defaults to the doppler zone property" do
      properties = {
        "doppler" => {
          "zone" => "my-zone"
        }
      }
      config = render_template(properties)

      expect(config["Zone"]).to eq("my-zone")
    end

    it "fails over to the az" do
      spec = InstanceSpec.new(az: "az2")
      config = render_template({}, spec: spec)

      expect(config["Zone"]).to eq("az2")
    end
  end

  describe "GRPC" do
    it "includes a port, certs, and cipher suites" do
      properties = {
        "loggregator" => {
          "tls" => {
            "cipher_suites" => "a:b"
          }
        },
        "doppler" => {
          "grpc_port" => 1111
        }
      }
      config = render_template(properties)

      expected_config = {
        "Port" => 1111,
        "KeyFile" => "/var/vcap/jobs/doppler/config/certs/doppler.key",
        "CertFile" => "/var/vcap/jobs/doppler/config/certs/doppler.crt",
        "CAFile" => "/var/vcap/jobs/doppler/config/certs/loggregator_ca.crt",
        "CipherSuites" => ["a", "b"]
      }
      expect(config["GRPC"]).to eq(expected_config)
    end
  end

  describe "MetronConfig" do
    it "includes a UDP and gRPC host port" do
      properties = {
        "metron_endpoint" => {
          "host" => "10.0.0.1",
          "dropsonde_port" => 1111,
          "grpc_port" => 2222,
        }
      }
      config = render_template(properties)

      metron_config = config["MetronConfig"]
      expect(metron_config["UDPAddress"]).to eq("10.0.0.1:1111")
      expect(metron_config["GRPCAddress"]).to eq("10.0.0.1:2222")
    end
  end

  def render_template(properties, spec: InstanceSpec.new)
    release_path = File.join(File.dirname(__FILE__), '../../../')
    release = Bosh::Template::Test::ReleaseDir.new(release_path)
    job = release.job('doppler')
    template = job.template('config/doppler.json')
    rendered = template.render(properties, spec: spec)

    JSON.parse(rendered)
  end
end
