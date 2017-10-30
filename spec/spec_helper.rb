require "json"
require "yaml"
require "bosh/template/test"

include Bosh::Template::Test

def render_template(props, job:, template:, spec: InstanceSpec.new, links: [])
  release_root = File.join(File.dirname(__FILE__), '../')
  release = Bosh::Template::Test::ReleaseDir.new(release_root)
  job = release.job(job)
  template = job.template(template)
  rendered = template.render(props, spec: spec, consumes: links)

  rendered.split("\n").each_with_object({}) do |str, h|
    if str != ""
      k, v = str.split("=")
      h[k.gsub("export ", "")] = v.gsub("\"", "")
    end
  end
end

def render_monit_config(properties, spec: InstanceSpec.new, links: [])
  release_path = File.join(File.dirname(__FILE__), '../')
  release = Bosh::Template::Test::ReleaseDir.new(release_path)
  job = release.job('metron_agent_windows')

  # Hack our monit template into the bosh templates
  job.instance_variable_get(:@templates)["monit"] = '../monit'
  template = job.template('../monit')
  p = template.instance_variable_get(:@template_path)
  p.gsub!("templates/monit", "monit")
  template.instance_variable_set(:@template_path, p)

  rendered = template.render(properties, spec: spec, consumes: links)

  JSON.parse(rendered)["processes"].first["env"]
end
