require "spec_helper"

RSpec.describe "Doppler Environment" do
  it "renders a complete environment" do
    properties = {
      "doppler" => {
        "container_metric_ttl_seconds" => 60,
        "disable_announce" => true,
        "grpc_port" => 1111,
        "health_addr" => "localhost:3333",
        "maxRetainedLogMessages" => 100,
        "pprof_port" => 2222,
        "sink_inactivity_timeout_seconds" => 10,
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
    config = render_template(
      properties,
      spec: spec,
      job: "doppler",
      template: "bin/environment.sh"
    )

    expected_config = {
      "CERT_DIR" => "$JOB_DIR/config/certs",
      "JOB_DIR" => "/var/vcap/jobs/doppler",
      "LOG_DIR" => "/var/vcap/sys/log/doppler",
      "PACKAGE_DIR" => "/var/vcap/packages/doppler",
      "PACKAGE_EXECUTABLE" => "doppler",
      "PIDFILE" => "${RUN_DIR}/doppler.pid",
      "PROFILE_EXECUTABLE" => "/var/vcap/packages/profiler/profiler.sh",
      "RUN_DIR" => "/var/vcap/sys/run/doppler",
      "AGENT_GRPC_ADDRESS" => "10.0.0.10:5555",
      "ROUTER_PORT" => "1111",
      "ROUTER_CERT_FILE" => "$CERT_DIR/doppler.crt",
      "ROUTER_KEY_FILE" => "$CERT_DIR/doppler.key",
      "ROUTER_CA_FILE" => "$CERT_DIR/loggregator_ca.crt",
      "ROUTER_CIPHER_SUITES" => "a,b",
      "ROUTER_MAX_RETAINED_LOG_MESSAGES" => "100",
      "ROUTER_CONTAINER_METRIC_TTL_SECONDS" => "60",
      "ROUTER_SINK_INACTIVITY_TIMEOUT_SECONDS" => "10",
      "ROUTER_PPROF_PORT" => "2222",
      "ROUTER_HEALTH_ADDR" => "localhost:3333",
    }
    expect(config).to eq(expected_config)
  end
end
