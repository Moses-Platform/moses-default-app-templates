require 'yaml'
expected_names = ARGV[2] ? [ARGV[2]] : ['backend', 'frontend']
containers = YAML.load_stream(File.read(ARGV[0])).compact
  .select { |d| d['kind'] == 'Deployment' }
  .flat_map { |d| d.dig('spec', 'template', 'spec', 'containers') }
  .select { |c| expected_names.include?(c['name']) }
raise "missing containers: #{expected_names}" unless containers.map { |c| c['name'] }.sort == expected_names.sort
containers.each do |c|
  values = c.fetch('env').select { |e| e['name'] == 'MOSES_MAX_BODY_BYTES' }
  raise "#{c['name']}: missing, duplicate or mismatched body cap: #{values}" unless values.length == 1 && values[0]['value'] == ARGV[1]
end
