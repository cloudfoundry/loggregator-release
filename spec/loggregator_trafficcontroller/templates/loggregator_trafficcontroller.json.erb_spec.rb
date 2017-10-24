require "json"
require "yaml"
require "bosh/template/test"

include Bosh::Template::Test

RSpec.describe "Traffic Controller JSON" do
  it "renders a full configuration" do
    properties = {
      "cc" => {
        "internal_service_hostname" => "cc.service.cf.internal",
        "tls_port" => 8888,
      },
      "doppler" => {
        "grpc_port" => 1111,
        "outgoing_port" => 4444,
      },
      "loggregator" => {
        "doppler" => {
          "addrs" => ["doppler.service.cf.internal"]
        },
        "outgoing_dropsonde_port" => 5555,
        "uaa" => {
          "client" => "some-client",
          "client_secret" => "some-secret"
        }
      },
      "metric_emitter" => {
        "interval" => 1,
      },
      "metron_endpoint" => {
        "dropsonde_port" => 2222,
        "grpc_port" => 3333,
        "host" => "10.0.0.1",
      },
      "ssl" => {
        "skip_cert_verify" => false,
      },
      "system_domain" => "bosh-lite.com",
      "traffic_controller" => {
        "pprof_port" => 6666,
        "health_addr" => "localhost:7777",
        "security_event_logging" => {
          "enabled" => true,
        }
      },
      "uaa" => {
        "internal_url" => "uaa.service.cf.internal"
      }
    }
    spec = InstanceSpec.new(ip: "10.0.0.250")
    config = render_template(properties, spec: spec)

    expected_config = {
      "IP" => "10.0.0.250",
      "RouterAddrs" => ["doppler.service.cf.internal:1111"],
      "RouterPort" => 4444,
      "OutgoingDropsondePort" => 5555,
      "GRPC" => {
        "Port" => 1111,
        "KeyFile" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/trafficcontroller.key",
        "CertFile" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/trafficcontroller.crt",
        "CAFile" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/loggregator_ca.crt"
      },
      "SystemDomain" => "bosh-lite.com",
      "PPROFPort" => 6666,
      "HealthAddr" => "localhost:7777",
      "UaaHost" => "uaa.service.cf.internal",
      "UaaClient" => "some-client",
      "UaaClientSecret" => "some-secret",
      "Agent" => {
        "UDPAddress" => "10.0.0.1:2222",
        "GRPCAddress" => "10.0.0.1:3333"
      },
      "MetricEmitterInterval" => 1,
      "CCTLSClientConfig" => {
        "CertFile" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/cc_trafficcontroller.crt",
        "KeyFile" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/cc_trafficcontroller.key",
        "CAFile" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/mutual_tls_ca.crt",
        "ServerName" => "cc.service.cf.internal",
      },
      "ApiHost" => "https://cc.service.cf.internal:8888",
      "UaaCACert" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/uaa_ca.crt",
      "SecurityEventLog" => "/var/vcap/sys/log/loggregator_trafficcontroller/loggregator_trafficcontroller_security_events.log",
      "SkipCertVerify" => false,
    }
    expect(config).to eq(expected_config)
  end

  describe "Router configuration" do
    it "consumes a Doppler link" do
      links = [Link.new(
        name: "doppler",
        instances: [LinkInstance.new(address: "doppler.service.cf.internal")],
        properties: {
          "doppler" => {
            "grpc_port" => 1111,
          }
        }
      )]
      config = render_template(required_properties, links: links)

      expect(config["RouterAddrs"]).to eq(["doppler.service.cf.internal:1111"])
    end

    it "uses an address property when no link is present" do
      properties = {
        "doppler" => {
          "grpc_port" => 1111,
        },
        "loggregator" => {
          "doppler" => {
            "addrs" => ["10.0.0.1"],
          },
          # required property of no importance here
          "uaa" => {
            "client_secret" => "secret"
          }
        }
      }
      config = render_template(required_properties.merge(properties))

      expect(config["RouterAddrs"]).to eq(["10.0.0.1:1111"])
    end
  end

  describe "UAA config" do
    it "configures a client" do
      properties = {
        "loggregator" => {
          "uaa" => {
            "client" => "some-client",
            "client_secret" => "some-secret"
          }
        }
      }
      config = render_template(required_properties.merge(properties))

      expect(config["UaaClient"]).to eq("some-client")
      expect(config["UaaClientSecret"]).to eq("some-secret")
      expect(config["UaaCACert"]).to be_nil
    end

    it "configures a client using an old property name" do
      properties = {
        "loggregator" => {
          "uaa_client_id" => "old-name",
          "uaa" => {
            "client" => "some-client",
            "client_secret" => "some-secret"
          }
        }
      }
      config = render_template(required_properties.merge(properties))

      expect(config["UaaClient"]).to eq("old-name")
    end

    it "adds a CA cert when the host is present" do
      properties = {
        "uaa" => {
          "internal_url" => "uaa.cf.service.internal"
        }
      }
      config = render_template(required_properties.merge(properties))

      expect(config["UaaCACert"]).to eq("/var/vcap/jobs/loggregator_trafficcontroller/config/certs/uaa_ca.crt")
    end
  end

  def render_template(properties, spec: InstanceSpec.new, links: [])
    release_path = File.join(File.dirname(__FILE__), "../../../")
    release = Bosh::Template::Test::ReleaseDir.new(release_path)
    job = release.job("loggregator_trafficcontroller")
    template = job.template("config/loggregator_trafficcontroller.json")
    rendered = template.render(properties, spec: spec, consumes: links)

    JSON.parse(rendered)
  end

  # These are the properties the Bosh spec file requires operators to
  # provide. The values are of no interest in the tests.
  def required_properties
    {
      "cc" => {
        "internal_service_hostname" => "cc.service.cf.internal"
      },
      "loggregator" => {
        "uaa" => {
          "client_secret" => "secret"
        }
      },
      "system_domain" => "bosh-lite.com",
    }
  end
end
