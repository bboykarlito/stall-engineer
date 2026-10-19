module stall-engineer

go 1.24.4

require github.com/bboykarlito/libstall v0.0.0

require github.com/bboykarlito/pi-in-stall v0.0.0

require go.yaml.in/yaml/v3 v3.0.5 // indirect

replace github.com/bboykarlito/libstall => ../libstall

replace github.com/bboykarlito/pi-in-stall => ../pi-in-stall
