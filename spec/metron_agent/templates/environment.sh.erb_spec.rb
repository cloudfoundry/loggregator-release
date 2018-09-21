require "spec_helper"

RSpec.describe "Metron Agent Environment" do
  it "renders a full environment" do
    spec = InstanceSpec.new(
        id: "some-id",
        ip: "127.0.0.1",
        name: "some-job",
    )
    properties = {
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
    config = render_template(
      properties,
      spec: spec,
      job: "metron_agent",
      template: "bin/environment.sh"
    )

    expected_config = {
      "AGENT_PORT" => "4444",
      "AGENT_CA_FILE" => "/var/vcap/jobs/metron_agent/config/certs/loggregator_ca.crt",
      "AGENT_CERT_FILE" => "/var/vcap/jobs/metron_agent/config/certs/metron_agent.crt",
      "AGENT_KEY_FILE" => "/var/vcap/jobs/metron_agent/config/certs/metron_agent.key",
      "AGENT_CIPHER_SUITES" => "a,b",
      "AGENT_DEPLOYMENT" => "some-deployment",
      "AGENT_ZONE" => "'some-az'",
      "AGENT_JOB" => "some-job",
      "AGENT_INDEX" => "some-id",
      "AGENT_IP" => "127.0.0.1",
      "AGENT_TAGS" => "deployment:some-deployment,job:some-job,index:some-id,ip:127.0.0.1,some-key:some-value",
      "AGENT_DISABLE_UDP" => "false",
      "AGENT_INCOMING_UDP_PORT" => "1111",
      "AGENT_HEALTH_ENDPOINT_PORT" => "3333",
      "AGENT_PPROF_PORT" => "2222",
      "ROUTER_ADDR" => "10.0.0.1:5555",
      "ROUTER_ADDR_WITH_AZ" => "'some-az.10.0.0.1:5555'",
    }
    expect(config).to eq(expected_config)
  end

  it "defaults to the job name of the spec" do
    spec = InstanceSpec.new(name: "some-name")
    config = render_template(
      {},
      spec: spec,
      job: "metron_agent",
      template: "bin/environment.sh"
    )

    expect(config["AGENT_JOB"]).to eq("some-name")
  end

  describe "Index" do
    it "defaults to the spec's ID" do
      spec = InstanceSpec.new(id: "some-id")
      config = render_template(
        {},
        spec: spec,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["AGENT_INDEX"]).to eq("some-id")
    end

    it "uses the spec's index when there is no ID" do
      spec = InstanceSpec.new(id: nil, index: 0)
      config = render_template(
        {},
        spec: spec,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["AGENT_INDEX"]).to eq("0")
    end
  end

  describe "Zone" do
    it "defaults to spec's az" do
      spec = InstanceSpec.new(az: "some-az")
      config = render_template(
        {},
        spec: spec,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["AGENT_ZONE"]).to eq("'some-az'")
    end

    it "uses the provided property" do
      spec = InstanceSpec.new(az: "some-az")
      prop = {
        "metron_agent" => {
          "zone" => "other-az",
        },
      }
      config = render_template(
        prop,
        spec: spec,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["AGENT_ZONE"]).to eq("'other-az'")
    end
  end

  describe "Deployment" do
    it "defaults to the spec's deployment" do
      spec = InstanceSpec.new(deployment: "some-deployment")
      config = render_template(
        {},
        spec: spec,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["AGENT_DEPLOYMENT"]).to eq("some-deployment")
    end

    it "uses the provided property" do
      spec = InstanceSpec.new(deployment: "some-deployment")
      properties = {
        "metron_agent" => {
          "deployment" => "other-deployment",
        },
      }
      config = render_template(
        properties,
        spec: spec,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["AGENT_DEPLOYMENT"]).to eq("other-deployment")
    end
  end

  describe "GRPC" do
    it "splits cipher suites separated by a colon" do
      properties = {
        "loggregator" => {
          "tls" => {
            "cipher_suites" => "a:b",
          },
        },
      }
      config = render_template(
        properties,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["AGENT_CIPHER_SUITES"]).to eq("a,b")
    end
  end

  describe "Tags" do
    it "appends arbitrary tags to bosh deployment metadata" do
      spec = InstanceSpec.new(
        deployment: "some-deployment",
        name: "some-job",
        id: "some-id",
        ip: "127.0.0.1",
      )
      properties = {
        "metron_agent" => {
          "tags" => {
            "other-tag" => "other-value",
          },
        },
      }
      config = render_template(
        properties,
        spec: spec,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expected_tags_str = "deployment:some-deployment,job:some-job,index:some-id,ip:127.0.0.1,other-tag:other-value"
      expect(config["AGENT_TAGS"]).to eq(expected_tags_str)
    end
  end

  describe "RouterAddr" do
    it "sets the addresses as a hostports" do
      properties = {
        "doppler" => {
          "addr" => "127.0.0.1",
          "grpc_port" => "9999",
        },
      }

      config = render_template(
        properties,
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["ROUTER_ADDR"]).to eq("127.0.0.1:9999")
    end
  end

  describe "Bosh DNS is enabled" do
    it "uses bosh links to populate RouterAddr and RouterAddrWithAZ" do
      pending "Links are not supported by bosh-template"

      instanceA = LinkInstance.new(
        name: 'doppler',
        az: 'az1',
        address: 'router.bosh.internal',
      )
      instanceB = LinkInstance.new(
        name: 'doppler',
        az: 'az2',
        address: 'router.bosh.internal',
      )
      link = Link.new(name: "doppler", instances: [instanceA, instanceB])

      properties = {"metron_agent" => {"bosh_dns" => true}}

      config = render_template(
        properties,
        links: [link],
        job: "metron_agent",
        template: "bin/environment.sh"
      )

      expect(config["ROUTER_ADDR"]).to eq("some-router-addr:8082")
      expect(config["ROUTER_ADDR_WITH_AZ"]).to eq("'az1.some-router-addr:8082'")
    end
  end
end
