package main

import (
	"archive/tar"
	"context"
	"encoding/json"
	"flag"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	dockerContainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type RuntimeData struct {
	Image      string
	Name       string
	Entrypoint string
}

func main() {
	// Load CLI args
	userDataLocation := flag.String("user-data", "testing/runtime-data.json", "Location of the runtime-data file")
	containerLocation := flag.String("container", "testing/container", "Location of the container directory")
	flag.Parse()

	// Load the runtime user-data
	file, err := ioutil.ReadFile(*userDataLocation)
	if err != nil {
		log.Fatalln("Failed to open user-data file", err)
	}

	runtimeData := RuntimeData{}

	err = json.Unmarshal([]byte(file), &runtimeData)
	if err != nil {
		log.Fatalln("Failed to parse runtime-data", err)
	}

	log.Println("Loading container with image " + runtimeData.Image)

	// Cleanup any previous container
	// In an actual environment this directory would be empty, but for testing it may not
	os.RemoveAll(*containerLocation)

	// Then recreate the container directory
	if err := os.MkdirAll(*containerLocation, 0755); err != nil {
		panic(err)
	}

	// Create the rootfs directory
	rootfs := filepath.Join(*containerLocation, "rootfs")
	if err := os.MkdirAll(rootfs, 0755); err != nil {
		panic(err)
	}

	// Generate an OCI runtime spec and place it in container directory
	ociSpec := generateOCISpec(runtimeData.Name, "/", "/var/run/netns/userappnet", []string{runtimeData.Entrypoint})

	ociSpecfile, _ := json.MarshalIndent(ociSpec, "", "	")

	_ = ioutil.WriteFile(filepath.Join(*containerLocation, "config.json"), ociSpecfile, 0644)

	err = exportDockerImage(runtimeData.Image, rootfs)
	if err != nil {
		log.Println("Failed to load Docker image", runtimeData.Image)
		panic(err)
	}

	// dodgy hack for creating resolv.conf
	err = os.WriteFile(filepath.Join(rootfs, "/etc/resolv.conf"), []byte("nameserver 8.8.8.8\n"), 0644)
	if err != nil {
		panic(err)
	}

	log.Println("Starting & attaching container")
	// Get the current working directory.
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	// Transfer stdin, stdout, and stderr to the new process
	// and also set target directory for the shell to start in.
	pa := os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Dir:   cwd,
	}

	// Start up a new shell.
	proc, err := os.StartProcess("/usr/local/bin/runsc", []string{"runsc", "run", "-bundle", *containerLocation, "user-container"}, &pa)
	if err != nil {
		panic(err)
	}

	// Wait until user exits the shell
	state, err := proc.Wait()
	if err != nil {
		panic(err)
	}
	log.Printf("<< Exited container: %s\n", state.String())
}

func exportDockerImage(image string, exportLocation string) error {
	// Load docker container
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	hasImage := false
	hostImages, err := cli.ImageList(ctx, types.ImageListOptions{})
	for _, hostImage := range hostImages {
		for _, tag := range hostImage.RepoTags {
			if tag == tag {
				hasImage = true
			}
		}
	}

	if hasImage {
		log.Println("Image exists already, skipping pull")
	} else {
		reader, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
		if err != nil {
			return err
		}
		io.Copy(os.Stdout, reader)
	}

	container, err := cli.ContainerCreate(ctx, &dockerContainer.Config{
		Image: image,
	}, nil, nil, nil, "")
	if err != nil {
		return err
	}

	exported_container, err := cli.ContainerExport(ctx, container.ID)
	if err != nil {
		return err
	}

	// Unpack the Docker image into the OCI container directory
	tarReader := tar.NewReader(exported_container)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		path := filepath.Join(exportLocation, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path, 0755); err != nil {
				return err
			}
			os.Chown(path, header.Uid, header.Gid)
		case tar.TypeReg:
			outFile, err := os.Create(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
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

	// Delete the staging container
	cli.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{})

	return nil
}

func containerCapabilities() []string {
	return []string{
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
	}
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
				Bounding:    containerCapabilities(),
				Effective:   containerCapabilities(),
				Inheritable: containerCapabilities(),
				Permitted:   containerCapabilities(),
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
