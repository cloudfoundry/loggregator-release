require "spec_helper"

RSpec.describe "Parity with Windows" do
  it "renders the same config" do
    linux_config = render_template(
      properties,
      spec: spec,
      job: "metron_agent",
      template: "bin/environment.sh"
    )
    windows_config = render_monit_config(properties, spec: spec)

    remove_known_difference!(windows_config)

    expect(linux_config).to eq(windows_config)
  end

  DIFFERENT_KEYS = [
    # Keys starting with __PIPE should be removed from the windows spec once
    # Loggregator removes syslog functionality from metron agent windows.
    "__PIPE_SYSLOG_HOST",
    "__PIPE_SYSLOG_PORT",
    "__PIPE_SYSLOG_TRANSPORT",
  ]

  def remove_known_difference!(windows_config)
    # The file paths of certs differ only by job name, e.g.,
    # /var/vcap/jobs/meton_agent/config/certs/ca.crt
    # vs
    # /var/vcap/jobs/meton_agent_windows/config/certs/ca.crt
    windows_config["AGENT_CA_FILE"] = windows_config["AGENT_CA_FILE"]
      .gsub("_windows", "")
    windows_config["AGENT_CERT_FILE"] = windows_config["AGENT_CERT_FILE"]
      .gsub("_windows", "")
    windows_config["AGENT_KEY_FILE"] = windows_config["AGENT_KEY_FILE"]
      .gsub("_windows", "")
    windows_config["AGENT_ZONE"] = "'#{windows_config["AGENT_ZONE"]}'"
    windows_config["ROUTER_ADDR_WITH_AZ"] = "'#{windows_config["ROUTER_ADDR_WITH_AZ"]}'"

    windows_config.reject! { |k| DIFFERENT_KEYS.include?(k) }
  end

  def spec
    InstanceSpec.new( id: "some-id", ip: "127.0.0.1", name: "some-job")
  end

  def properties
    {
      "doppler" => {
          "addr" => "10.0.0.1",
          "grpc_port" => 5555,
      },
      "metron_agent" => {
        "zone" => "some-az",
        "deployment" => "some-deployment",
        "grpc_port" => 4444,
        "listening_port" => 1111,
        "disable_udp" => false,
        "pprof_port" => 2222,
        "health_port" => 3333,
        "tags" => {
            "some-key" => "some-value"
        }
      },
      "loggregator" => {
          "tls" => {
              "cipher_suites" => "a:b"
          }
      }
    }
  end
end
