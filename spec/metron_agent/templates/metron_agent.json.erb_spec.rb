require "json"
require "yaml"
require "bosh/template/test"

include Bosh::Template::Test

RSpec.describe "Metron Agent JSON" do
  it "renders a full JSON configuration" do
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
    config = render_template(properties, spec: spec)

    expected_config = {
      "Index" => "some-id",
      "Job" => "some-job",
      "Zone" => "some-az",
      "Deployment" => "some-deployment",
      "IP" => "127.0.0.1",
      "Tags" => {
        "deployment" => "some-deployment",
        "job" => "some-job",
        "index" => "some-id",
        "ip" => "127.0.0.1",
        "some-key" => "some-value",
      },
      "IncomingUDPPort" => 1111,
      "DisableUDP" => false,
      "PPROFPort" => 2222,
      "HealthEndpointPort" => 3333,
      "GRPC" => {
        "Port" => 4444,
        "KeyFile" => "/var/vcap/jobs/metron_agent/config/certs/metron_agent.key",
        "CertFile" => "/var/vcap/jobs/metron_agent/config/certs/metron_agent.crt",
        "CAFile" => "/var/vcap/jobs/metron_agent/config/certs/loggregator_ca.crt",
        "CipherSuites" => ["a", "b"]

      },
      "DopplerAddr" => "10.0.0.1:5555",
      "DopplerAddrWithAZ" => "some-az.10.0.0.1:5555",
    }
    expect(config).to eq(expected_config)
  end

  it "defaults to the job name of the spec" do
    spec = InstanceSpec.new(name: "some-name")
    config = render_template({}, spec: spec)

    expect(config["Job"]).to eq("some-name")
  end

  describe "Index" do
    it "defaults to the spec's ID" do
      spec = InstanceSpec.new(id: "some-id")
      config = render_template({}, spec: spec)

      expect(config["Index"]).to eq("some-id")
    end

    it "uses the spec's index when there is no ID" do
      spec = InstanceSpec.new(id: nil, index: 0)
      config = render_template({}, spec: spec)

      expect(config["Index"]).to eq("0")
    end
  end

  describe "Zone" do
    it "defaults to spec's az" do
      spec = InstanceSpec.new(az: "some-az")
      config = render_template({}, spec: spec)

      expect(config["Zone"]).to eq("some-az")
    end

    it "uses the provided property" do
      spec = InstanceSpec.new(az: "some-az")
      prop = {
        "metron_agent" => {
          "zone" => "other-az",
        },
      }
      config = render_template(prop, spec: spec)

      expect(config["Zone"]).to eq("other-az")
    end
  end

  describe "Deployment" do
    it "defaults to the spec's deployment" do
      spec = InstanceSpec.new(deployment: "some-deployment")
      config = render_template({}, spec: spec)

      expect(config["Deployment"]).to eq("some-deployment")
    end

    it "uses the provided property" do
      spec = InstanceSpec.new(deployment: "some-deployment")
      properties = {
        "metron_agent" => {
          "deployment" => "other-deployment",
        },
      }
      config = render_template(properties, spec: spec)

      expect(config["Deployment"]).to eq("other-deployment")
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
      config = render_template(properties)

      expect(config["GRPC"]["CipherSuites"]).to eq(["a", "b"])
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
      config = render_template(properties, spec: spec)

      expected_tags = {
        "deployment" => "some-deployment",
        "job" => "some-job",
        "index" => "some-id",
        "ip" => "127.0.0.1",
        "other-tag" => "other-value",
      }
      expect(config["Tags"]).to eq(expected_tags)
    end
  end

  describe "DopplerAddr" do
    it "sets the addresses as a hostports" do
      properties = {
        "doppler" => {
          "addr" => "127.0.0.1",
          "grpc_port" => "9999",
        },
      }

      config = render_template(properties)

      expect(config["DopplerAddr"]).to eq("127.0.0.1:9999")
    end
  end

  describe "Bosh DNS is enabled" do
    it "uses bosh links to populate DopplerAddr and DopplerAddrWithAZ" do
      pending "Links are not supported by bosh-template"

      instanceA = LinkInstance.new(
        name: 'doppler',
        az: 'az1',
        address: 'doppler.bosh.internal',
      )
      instanceB = LinkInstance.new(
        name: 'doppler',
        az: 'az2',
        address: 'doppler.bosh.internal',
      )
      link = Link.new(name: "doppler", instances: [instanceA, instanceB])

      properties = {"metron_agent" => {"bosh_dns" => true}}

      config = render_template(properties, links: [link])

      expect(config['DopplerAddr']).to eq('some-doppler-addr:8082')
      expect(config['DopplerAddrWithAZ']).to eq('az1.some-doppler-addr:8082')
    end
  end
end

def render_template(properties, spec: InstanceSpec.new, links: [])
  release_path = File.join(File.dirname(__FILE__), '../../../')
  release = ReleaseDir.new(release_path)
  job = release.job('metron_agent')
  template = job.template('config/metron_agent.json')
  rendered = template.render(properties, spec: spec, consumes: links)

  JSON.parse(rendered)
end
