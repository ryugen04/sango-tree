package process

import "fmt"

// DockerOptions はDockerサービス起動のオプション
type DockerOptions struct {
	Name    string
	Image   string
	Port    int
	Env     map[string]string
	Volumes []string
}

// BuildDockerArgs はdocker runのコマンド引数を組み立てる
func BuildDockerArgs(opts DockerOptions) []string {
	args := []string{
		"run", "-d",
		"--name", fmt.Sprintf("grove-%s", opts.Name),
		"-p", fmt.Sprintf("%d:%d", opts.Port, opts.Port),
	}

	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	for _, vol := range opts.Volumes {
		args = append(args, "-v", vol)
	}

	args = append(args, opts.Image)
	return args
}
