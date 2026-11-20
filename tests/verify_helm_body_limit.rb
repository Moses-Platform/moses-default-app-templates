require 'yaml'
containers = YAML.load_stream(File.read(ARGV[0])).compact
  .select { |d| d['kind'] == 'Deployment' }
  .flat_map { |d| d.dig('spec', 'template', 'spec', 'containers') }
  .select { |c| ['frontend', 'backend'].include?(c['name']) }
raise 'missing frontend/backend' unless containers.map { |c| c['name'] }.sort == ['backend', 'frontend']
containers.each do |c|
  values = c.fetch('env').select { |e| e['name'] == 'MOSES_MAX_BODY_BYTES' }
  raise "#{c['name']}: missing, duplicate or mismatched body cap: #{values}" unless values.length == 1 && values[0]['value'] == ARGV[1]
end
