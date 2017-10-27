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


