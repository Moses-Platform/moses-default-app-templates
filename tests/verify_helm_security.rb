require 'yaml'
docs = YAML.load_stream(File.read(ARGV[0])).compact
workloads = docs.select { |d| ['Deployment','StatefulSet'].include?(d['kind']) }
raise 'no workloads' if workloads.empty?
workloads.each do |d|
  pod = d.fetch('spec').fetch('template').fetch('spec')
  raise 'API token mounted' unless pod['automountServiceAccountToken'] == false
  psc = pod.fetch('securityContext')
  raise 'pod root/unsafe seccomp' unless psc['runAsNonRoot'] && psc.dig('seccompProfile','type') == 'RuntimeDefault'
  pod.fetch('containers').each do |c|
    sc = c.fetch('securityContext')
    uid, gid = case c['name']
    when 'postgresql' then [70,70]
    when 'redis' then [999,1000]
    when 'frontend' then [101,101]
    else ARGV[1] == 'frontend-template' ? [101,101] : [1000,1000]
    end
    raise "wrong image user #{c['name']}" unless sc['runAsUser']==uid && sc['runAsGroup']==gid && psc['runAsUser']==uid && psc['runAsGroup']==gid && psc['fsGroup']==gid
    raise 'container root' unless sc['runAsNonRoot']
    raise 'escalation allowed' unless sc['allowPrivilegeEscalation']==false
    raise 'capabilities retained' unless sc.dig('capabilities','drop')==['ALL']
    raise 'unsafe seccomp' unless sc.dig('seccompProfile','type')=='RuntimeDefault'
    raise 'writable root' unless sc['readOnlyRootFilesystem']==true
    mounts=c.fetch('volumeMounts')
    paths=mounts.map { |m| m['mountPath'] }
    raise 'missing tmp' unless paths.include?('/tmp')
    required=case c['name']; when 'postgresql' then ['/var/run/postgresql','/var/lib/postgresql/data']; when 'redis' then ['/data']; else []; end
    required.each { |p| raise "missing #{p}" unless paths.include?(p) }
    mounts.each { |m| raise 'volume missing' unless pod.fetch('volumes').any? { |v| v['name']==m['name'] } }
    if c['name']=='redis'
      raise 'redis port missing' unless c.fetch('ports').any? { |p| p['containerPort']==6379 && p['name']=='redis' }
      raise 'no auth' unless c.fetch('args').join(' ').include?('--requirepass') && c.fetch('env').any? { |e| e['name']=='REDIS_PASSWORD' && e.dig('valueFrom','secretKeyRef','key')=='password' }
    end
    if c['name']=='frontend' || ARGV[1]=='frontend-template'
      raise 'frontend port changed' unless c.fetch('ports').first['containerPort']==8080
      %w[startupProbe livenessProbe readinessProbe].each { |p| raise 'probe drift' unless c.dig(p,'httpGet','path')=='/health' }
    end
  end
end

# Services must still select their pods and resolve each target to a real port.
docs.select { |d| d['kind']=='Service' }.each do |svc|
  selector=svc.fetch('spec').fetch('selector')
  targets=workloads.select { |w| labels=w.dig('spec','template','metadata','labels'); selector.all? { |k,v| labels[k]==v } }
  raise 'service has no workload' if targets.empty?
  svc.fetch('spec').fetch('ports').each do |port|
    target=port['targetPort'] || port['port']
    targets.each do |w|
      ports=w.dig('spec','template','spec','containers').flat_map { |c| c['ports'] || [] }
      raise 'service target port drift' unless ports.any? { |p| target.is_a?(Integer) ? p['containerPort']==target : p['name']==target }
    end
  end
end
