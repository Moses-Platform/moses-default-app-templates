# Verify rendered references without ever putting credential bytes in Helm values.
require 'yaml'
docs = YAML.load_stream(File.read(ARGV[0])).compact
kind, key = ARGV[1], ARGV[2]
password_envs = %w[POSTGRES_PASSWORD DATABASE_PASSWORD DB_PASSWORD REDIS_PASSWORD]
docs.select { |d| d['kind'] == 'Secret' }.each do |d|
  fields = (d['data'] || {}).merge(d['stringData'] || {}).keys
  raise 'chart rendered a bundled/signing credential Secret' if fields.any? { |k| k.downcase.include?('password') || k == 'MOSES_INTERAPP_SECRET' } || d.dig('metadata', 'name') == 'bundled-credentials'
end
containers = docs.select { |d| %w[Deployment StatefulSet].include?(d['kind']) }.flat_map { |d| d.fetch('spec').fetch('template').fetch('spec').fetch('containers') }
raise 'no containers' if containers.empty?
servers = containers.select { |c| %w[postgresql redis].include?(c['name']) }
raise 'wrong bundled server count' unless servers.size == (kind == 'none' ? 0 : 1)
containers.each do |c|
  env = c.fetch('env', [])
  raise 'explicit signing-key override' if env.any? { |e| e['name'] == 'MOSES_INTERAPP_SECRET' }
  expected = if ARGV[3] == 'fullstack-chat' && %w[frontend web ui].include?(c['name'])
    # Chat deliberately restricts secret delivery to the backend. Preserve it.
    raise 'chat frontend received secrets' unless (c['envFrom'] || []).empty?
    raise 'chat frontend received database URL' if env.any? { |e| e['name'] == 'DATABASE_URL' }
    []
  elsif c['name'] == 'postgresql'
    %w[POSTGRES_PASSWORD]
  elsif c['name'] == 'redis'
    raise 'Redis AUTH expansion lost' unless c['args'] == ['valkey-server', '--requirepass', '$(REDIS_PASSWORD)']
    %w[REDIS_PASSWORD]
  else
    expected_sources = [
      {'secretRef' => {'name' => 'app-extra'}},
      {'configMapRef' => {'name' => 'app-config'}},
      {'secretRef' => {'name' => 'app-runtime'}},
      {'secretRef' => {'name' => 'canonical-interapp'}}
    ]
    raise 'envFrom order or sources changed' unless c['envFrom'] == expected_sources
    case kind
    when 'postgresql'
      url = env.find { |e| e['name'] == 'DATABASE_URL' }
      raise 'DATABASE_URL lost runtime expansion' unless url && url['value'].include?(':$(DATABASE_PASSWORD)@')
      raise 'DATABASE_PASSWORD must precede URL' unless env.index { |e| e['name'] == 'DATABASE_PASSWORD' } < env.index(url)
      %w[DATABASE_PASSWORD DB_PASSWORD]
    when 'redis' then %w[REDIS_PASSWORD]
    else []
    end
  end
  actual = env.select { |e| password_envs.include?(e['name']) }
  raise 'password env missing or duplicated' unless actual.map { |e| e['name'] }.sort == expected.sort
  actual.each do |e|
    raise 'password is not the canonical reference' unless e == {'name' => e['name'], 'valueFrom' => {'secretKeyRef' => {'name' => 'bundled-credentials', 'key' => key}}}
  end
end
