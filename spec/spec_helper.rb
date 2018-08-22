require "json"
require "yaml"
require "bosh/template/test"

include Bosh::Template::Test

class ::Bosh::Template::Test::Template
  def render(manifest_properties_hash, spec: InstanceSpec.new, consumes: [])
    spec_hash = {}
    spec_hash['properties'] = hash_with_defaults(manifest_properties_hash)
    sanitized_hash_with_spec = spec_hash.merge(spec.to_h)
    sanitized_hash_with_spec['links'] = links_hash(consumes)

    binding = Bosh::Template::EvaluationContext.new(
      sanitized_hash_with_spec,
      Bosh::Template::ManualLinkDnsEncoder.new('link-address'),
    ).get_binding
    raise "No such file at #{@template_path}" unless File.exist?(@template_path)
    ERB.new(File.read(@template_path), safe_level = nil, trim_mode = '-').result(binding)
  end
end

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
