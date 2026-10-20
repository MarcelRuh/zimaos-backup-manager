package composebackup

import (
	"context"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/dockerapi"
)

// Project is one Docker Compose app with its AppData folder mapping.
type Project struct {
	Name            string   `json:"name"`
	ComposeHost     string   `json:"compose_host"`
	ComposeLocal    string   `json:"compose_local"`
	AppDataName     string   `json:"appdata_name"`
	AppDataLocal    string   `json:"appdata_local"`
	Containers      []string `json:"containers"`
	ComposeReadable bool     `json:"compose_readable"`
	AppDataWritable bool     `json:"appdata_writable"`
}

type Discoverer interface {
	ListContainers(ctx context.Context) ([]dockerapi.Container, error)
}

// DiscoverProjects groups containers by compose config file and maps AppData dirs.
func DiscoverProjects(ctx context.Context, docker Discoverer, appDataHostRoot, appDataLocalRoot, composeHostRoot, composeLocalRoot string) ([]Project, error) {
	containers, err := docker.ListContainers(ctx)
	if err != nil {
		return nil, err
	}
	appDataHostRoot = cleanSlash(appDataHostRoot)
	appDataLocalRoot = cleanSlash(appDataLocalRoot)
	composeHostRoot = cleanSlash(composeHostRoot)
	composeLocalRoot = cleanSlash(composeLocalRoot)

	type agg struct {
		composeHost string
		name        string
		appData     string
		members     map[string]struct{}
	}
	byCompose := map[string]*agg{}

	for _, c := range containers {
		files := c.ComposeConfigFiles()
		if len(files) == 0 {
			continue
		}
		composeHost := cleanSlash(files[0])
		key := composeHost
		a := byCompose[key]
		if a == nil {
			name := c.ComposeProject()
			if name == "" {
				name = path.Base(path.Dir(composeHost))
			}
			if name == "" || name == "." || name == "/" {
				name = path.Base(composeHost)
			}
			a = &agg{
				composeHost: composeHost,
				name:        name,
				members:     map[string]struct{}{},
			}
			byCompose[key] = a
		}
		a.members[c.Name()] = struct{}{}
		if a.appData == "" {
			a.appData = appDataNameFromMounts(c.Mounts, appDataHostRoot)
		}
		if a.appData == "" && appDataHostRoot != "" {
			candidate := path.Join(appDataHostRoot, a.name)
			_ = candidate
			// Prefer explicit mount mapping; name fallback applied later if folder exists.
		}
	}

	out := make([]Project, 0, len(byCompose))
	for _, a := range byCompose {
		appName := a.appData
		if appName == "" {
			appName = a.name
		}
		members := make([]string, 0, len(a.members))
		for m := range a.members {
			members = append(members, m)
		}
		sort.Strings(members)

		composeLocal := rewritePath(a.composeHost, composeHostRoot, composeLocalRoot)
		appLocal := ""
		if appDataLocalRoot != "" && appName != "" {
			appLocal = path.Join(appDataLocalRoot, appName)
		}
		out = append(out, Project{
			Name:            a.name,
			ComposeHost:     a.composeHost,
			ComposeLocal:    composeLocal,
			AppDataName:     appName,
			AppDataLocal:    appLocal,
			Containers:      members,
			ComposeReadable: fileReadable(composeLocal),
			AppDataWritable: dirWritable(appLocal),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func appDataNameFromMounts(mounts []dockerapi.Mount, appDataHostRoot string) string {
	appDataHostRoot = cleanSlash(appDataHostRoot)
	if appDataHostRoot == "" {
		return ""
	}
	prefix := appDataHostRoot + "/"
	for _, m := range mounts {
		src := cleanSlash(m.Source)
		if src == appDataHostRoot {
			continue
		}
		if strings.HasPrefix(src, prefix) {
			rest := strings.TrimPrefix(src, prefix)
			name := strings.Split(rest, "/")[0]
			if name != "" && name != "." && name != ".." {
				return name
			}
		}
	}
	return ""
}

func rewritePath(hostPath, hostRoot, localRoot string) string {
	hostPath = cleanSlash(hostPath)
	hostRoot = cleanSlash(hostRoot)
	localRoot = cleanSlash(localRoot)
	if hostPath == "" {
		return ""
	}
	if hostRoot != "" && localRoot != "" {
		if hostPath == hostRoot {
			return localRoot
		}
		if strings.HasPrefix(hostPath, hostRoot+"/") {
			return path.Join(localRoot, strings.TrimPrefix(hostPath, hostRoot+"/"))
		}
	}
	// Fallbacks for common CasaOS layouts when roots are configured.
	return hostPath
}

func cleanSlash(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}
