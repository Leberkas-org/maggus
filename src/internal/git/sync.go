package git

func (o *ops) Fetch(dir string) error {
	return o.cmd.Run(dir, "fetch")
}

func (o *ops) Pull(dir string) error {
	return o.cmd.Run(dir, "pull", "--rebase")
}

func (o *ops) RemoteExists(dir string) bool {
	out, err := o.cmd.Output(dir, "remote")
	if err != nil {
		return false
	}
	return out != ""
}

func (o *ops) RepoURL(dir string) string {
	url, err := o.cmd.Output(dir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return url
}
