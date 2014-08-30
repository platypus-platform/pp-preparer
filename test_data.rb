require 'json'

app = "slug"
node = {
  "cluster" => "development"
}
hostname = `hostname`.chomp

cmd = "curl -X PUT localhost:8500/v1/kv/nodes/#{hostname}/#{app} -d '#{node.to_json}'"
puts cmd
`#{cmd}`

version = {
  "e928e5ad8814441e7c503d7f6c9e55d72584c006" => 'prep'
}

# Deploy config cannot be changed by developers. It requires priviliged access
# to change, since it allows you to run stuff has root in arbitrary locations.
deploy_config = {
  'basedir' => '/tmp/slug',
  'runas'   => nil
}

cmd = "curl -X PUT localhost:8500/v1/kv/clusters/#{node["cluster"]}/versions -d '#{version.to_json}'"
puts cmd
`#{cmd}`

cmd = "curl -X PUT localhost:8500/v1/kv/clusters/#{node["cluster"]}/deploy_config -d '#{deploy_config.to_json}'"
puts cmd
`#{cmd}`
