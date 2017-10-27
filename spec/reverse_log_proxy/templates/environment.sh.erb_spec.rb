require "spec_helper"

RSpec.describe "Reverse Log Proxy Environment" do
  it "renders a full environment" do
    properties = {
      "reverse_log_proxy" => {
        "egress" => {
          "port" => "1111",
        },
        "pprof" => {
          "port" => "2222",
        },
        "health_addr" => "localhost:3333",
      },
      "loggregator" => {
        "tls" => {
          "reverse_log_proxy" => {
            "key" => "/var/vcap/jobs/reverse_log_proxy/config/certs/reverse_log_proxy.key",
            "cert" => "/var/vcap/jobs/reverse_log_proxy/config/certs/reverse_log_proxy.crt",
          },
          "ca_cert" => "/var/vcap/jobs/reverse_log_proxy/config/certs/mutual_tls_ca.crt",
          "cipher_suites" => "a:b",
        },
        "doppler" => {
          "addrs" => ["dopper-1.service.cf.internal", "doppler-2.service.cf.internal"],
          "grpc_port" => 4444,
        },
      },
      "metron_endpoint" => {
        "host" => "10.0.0.1",
        "grpc_port" => 5555,
      },
      "metric_emitter" => {
        "interval" => "15m",
      }
    }
    config = render_template(
      properties,
      job: "reverse_log_proxy",
      template: "bin/environment.sh"
    )

    expected_config = {
      "RLP_PORT" => "1111",
      "RLP_CERT_FILE" => "/var/vcap/jobs/reverse_log_proxy/config/certs/reverse_log_proxy.crt",
      "RLP_KEY_FILE" => "/var/vcap/jobs/reverse_log_proxy/config/certs/reverse_log_proxy.key",
      "RLP_CA_FILE" => "/var/vcap/jobs/reverse_log_proxy/config/certs/mutual_tls_ca.crt",
      "RLP_CIPHER_SUITES" => "a,b",
      "RLP_PPROF_PORT" => "2222",
      "RLP_HEALTH_ADDR" => "localhost:3333",
      "RLP_METRIC_EMITTER_INTERVAL" => "15m",
      "ROUTER_ADDRS" => "dopper-1.service.cf.internal:4444,doppler-2.service.cf.internal:4444",
      "AGENT_ADDR" => "10.0.0.1:5555",
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
        {},
        links: links,
        job: "reverse_log_proxy",
        template: "bin/environment.sh"
      )

      expect(config["ROUTER_ADDRS"]).to eq("doppler-1.service.cf.internal:1111,doppler-2.service.cf.internal:1111")
    end
  end
end
