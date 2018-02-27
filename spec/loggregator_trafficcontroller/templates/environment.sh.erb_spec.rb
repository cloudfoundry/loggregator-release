require "spec_helper"

RSpec.describe "Traffic Controller Environment" do
  it "renders a full environment" do
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
        "interval" => "1m",
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
        "disable_access_control" => true,
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
    config = render_template(
      properties,
      spec: spec,
      job: "loggregator_trafficcontroller",
      template: "bin/environment.sh"
    )

    expected_config = {
      "AGENT_UDP_ADDRESS" => "10.0.0.1:2222",
      "AGENT_GRPC_ADDRESS" => "10.0.0.1:3333",
      "ROUTER_ADDRS" => "doppler.service.cf.internal:1111",
      "ROUTER_CA_FILE" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/loggregator_ca.crt",
      "ROUTER_CERT_FILE" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/trafficcontroller.crt",
      "ROUTER_KEY_FILE" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/trafficcontroller.key",
      "CC_CERT_FILE" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/cc_trafficcontroller.crt",
      "CC_KEY_FILE" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/cc_trafficcontroller.key",
      "CC_CA_FILE" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/mutual_tls_ca.crt",
      "CC_SERVER_NAME" => "cc.service.cf.internal",
      "TRAFFIC_CONTROLLER_IP" => "10.0.0.250",
      "TRAFFIC_CONTROLLER_API_HOST" => "https://cc.service.cf.internal:8888",
      "TRAFFIC_CONTROLLER_OUTGOING_DROPSONDE_PORT" => "5555",
      "TRAFFIC_CONTROLLER_SYSTEM_DOMAIN" => "bosh-lite.com",
      "TRAFFIC_CONTROLLER_SKIP_CERT_VERIFY" => "false",
      "TRAFFIC_CONTROLLER_UAA_HOST" => "uaa.service.cf.internal",
      "TRAFFIC_CONTROLLER_UAA_CLIENT" => "some-client",
      "TRAFFIC_CONTROLLER_UAA_CLIENT_SECRET" => "'some-secret'",
      "TRAFFIC_CONTROLLER_UAA_CA_CERT" => "/var/vcap/jobs/loggregator_trafficcontroller/config/certs/uaa_ca.crt",
      "TRAFFIC_CONTROLLER_SECURITY_EVENT_LOG" => "/var/vcap/sys/log/loggregator_trafficcontroller/loggregator_trafficcontroller_security_events.log",
      "TRAFFIC_CONTROLLER_PPROF_PORT" => "6666",
      "TRAFFIC_CONTROLLER_METRIC_EMITTER_INTERVAL" => "1m",
      "TRAFFIC_CONTROLLER_HEALTH_ADDR" => "localhost:7777",
      "TRAFFIC_CONTROLLER_DISABLE_ACCESS_CONTROL" => "true",
    }
    expect(config).to eq(expected_config)
  end

  describe "Router configuration" do
    it "consumes a Doppler link" do
      links = [Link.new(
        name: "doppler",
        instances: [
          LinkInstance.new(address: "doppler-1.service.cf.internal"),
          LinkInstance.new(address: "doppler-2.service.cf.internal"),
        ],
        properties: {
          "doppler" => {
            "grpc_port" => 1111,
          }
        }
      )]
      config = render_template(
        required_properties,
        links: links,
        job: "loggregator_trafficcontroller",
        template: "bin/environment.sh"
      )

      expect(config["ROUTER_ADDRS"]).to eq("doppler-1.service.cf.internal:1111,doppler-2.service.cf.internal:1111")
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
      config = render_template(
        required_properties.merge(properties),
        job: "loggregator_trafficcontroller",
        template: "bin/environment.sh"
      )

      expect(config["ROUTER_ADDRS"]).to eq("10.0.0.1:1111")
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
      config = render_template(
        required_properties.merge(properties),
        job: "loggregator_trafficcontroller",
        template: "bin/environment.sh"
      )

      expect(config["TRAFFIC_CONTROLLER_UAA_CLIENT"]).to eq("some-client")
      expect(config["TRAFFIC_CONTROLLER_UAA_CLIENT_SECRET"]).to eq("'some-secret'")
      expect(config["TRAFFIC_CONTROLLER_UAA_CA_CERT"]).to be_nil
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
      config = render_template(
        required_properties.merge(properties),
        job: "loggregator_trafficcontroller",
        template: "bin/environment.sh"
      )

      expect(config["TRAFFIC_CONTROLLER_UAA_CLIENT"]).to eq("old-name")
    end

    it "adds a CA cert when the host is present" do
      properties = {
        "uaa" => {
          "internal_url" => "uaa.cf.service.internal"
        }
      }
      config = render_template(
        required_properties.merge(properties),
        job: "loggregator_trafficcontroller",
        template: "bin/environment.sh"
      )

      expect(config["TRAFFIC_CONTROLLER_UAA_CA_CERT"]).to eq("/var/vcap/jobs/loggregator_trafficcontroller/config/certs/uaa_ca.crt")
    end
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
