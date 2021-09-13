package main

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"

	"github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type UserData struct {
	Image    string
	Hostname string
}

func main() {
	file, _ := ioutil.ReadFile("testing/user-data.json")

	user_data := UserData{}

	_ = json.Unmarshal([]byte(file), &user_data)

	log.Println("Loading container with image " + user_data.Image)

	os.RemoveAll("testing/container")

	container_location := "testing/container"
	if err := os.MkdirAll(container_location, 0755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(container_location+"/rootfs", 0755); err != nil {
		panic(err)
	}

	ociSpec := generateOCISpec(user_data.Hostname, "/", "", []string{"/bin/echo", "HELLO", "FROM", "CONTAINER"})

	ociSpecfile, _ := json.MarshalIndent(ociSpec, "", "	")

	_ = ioutil.WriteFile("testing/container/config.json", ociSpecfile, 0644)

	// return //testinggggg

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	reader, err := cli.ImagePull(ctx, user_data.Image, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	io.Copy(os.Stdout, reader)

	container, err := cli.ContainerCreate(ctx, &dockerContainer.Config{
		Image: user_data.Image,
	}, nil, nil, nil, "")
	if err != nil {
		panic(err)
	}

	exported_container, err := cli.ContainerExport(ctx, container.ID)
	if err != nil {
		panic(err)
	}

	dst := "testing/container/rootfs"

	tarReader := tar.NewReader(exported_container)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("ExtractTarGz: Next() failed: %s", err.Error())
		}

		path := dst + "/" + header.Name

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path, 0755); err != nil {
				log.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
			}
			os.Chown(path, header.Uid, header.Gid)
		case tar.TypeReg:
			outFile, err := os.Create(path)
			if err != nil {
				log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
			outFile.Close()
			os.Chown(path, header.Uid, header.Gid)
			os.Chmod(path, fs.FileMode(header.Mode))
		case tar.TypeSymlink, tar.TypeLink:
			os.Symlink(header.Linkname, path)

		default:
			log.Printf(
				"ExtractTarGz: uknown type: %b in %s",
				header.Typeflag,
				path)
		}

	}

	cli.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{})
}

func generateOCISpec(hostname string, cwd string, netns string, args []string) *specs.Spec {
	spec := &specs.Spec{
		Version: "1.0.0",
		Process: &specs.Process{
			Terminal: true,
			User: specs.User{
				UID: 0,
				GID: 0,
			},
			Args: args,
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			Cwd: cwd,
			Capabilities: &specs.LinuxCapabilities{
				Bounding: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				Effective: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				Inheritable: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				Permitted: []string{
					"CAP_CHOWN",
					"CAP_DAC_OVERRIDE",
					"CAP_FSETID",
					"CAP_FOWNER",
					"CAP_MKNOD",
					"CAP_NET_RAW",
					"CAP_SETGID",
					"CAP_SETUID",
					"CAP_SETFCAP",
					"CAP_SETPCAP",
					"CAP_NET_BIND_SERVICE",
					"CAP_SYS_CHROOT",
					"CAP_KILL",
					"CAP_AUDIT_WRITE",
				},
				// TODO(gvisor.dev/issue/3166): support ambient capabilities
			},
			Rlimits: []specs.POSIXRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: 1024,
					Soft: 1024,
				},
			},
		},
		Root: &specs.Root{
			Path:     "rootfs",
			Readonly: false,
		},
		Hostname: hostname,
		Mounts: []specs.Mount{
			{
				Destination: "/proc",
				Type:        "proc",
				Source:      "proc",
			},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options: []string{
					"nosuid",
					"noexec",
					"nodev",
					"ro",
				},
			},
		},
		Linux: &specs.Linux{
			Namespaces: []specs.LinuxNamespace{
				{
					Type: "pid",
				},
				{
					Type: "network",
					Path: netns,
				},
				{
					Type: "ipc",
				},
				{
					Type: "uts",
				},
				{
					Type: "mount",
				},
			},
		},
	}

	return spec
}
