package dockerstacks

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestConvertDockerRunCompose(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		service  string
		compose  string
		warnings []string // "code flag"
	}{
		{
			name:    "web server with a named volume",
			command: "docker run -d --name web -p 8080:80 -v data:/usr/share/nginx/html --restart unless-stopped nginx:1.27",
			service: "web",
			compose: `services:
  web:
    image: nginx:1.27
    container_name: web
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - data:/usr/share/nginx/html
volumes:
  data: {}
`,
		},
		{
			name:    "combined short flags, environment, literal dollar",
			command: `docker run -dp 80:80 -e TZ=Europe/Berlin -e PASS='pa$word' -e KEY -e FLAG=true -e TZ=UTC nginx`,
			service: "nginx",
			compose: `services:
  nginx:
    image: nginx
    ports:
      - "80:80"
    environment:
      TZ: UTC
      PASS: pa$$word
      KEY:
      FLAG: "true"
`,
		},
		{
			name:    "values attached to short flags",
			command: "docker run -p81:81 -v/srv/a:/a -e=B=2 -itd nginx",
			service: "nginx",
			compose: `services:
  nginx:
    image: nginx
    ports:
      - "81:81"
    volumes:
      - /srv/a:/a
    environment:
      B: "2"
    tty: true
    stdin_open: true
`,
		},
		{
			name:    "powershell with backticks, pwd and rm",
			command: "docker run -it --rm `\n  -v ${PWD}/data:/data `\n  -v $PWD/x:/x -v \"$(pwd)\":/app `\n  ubuntu:24.04 bash -c \"echo hi\"",
			service: "ubuntu",
			compose: `services:
  ubuntu:
    image: ubuntu:24.04
    command:
      - bash
      - -c
      - echo hi
    volumes:
      - ./data:/data
      - ./x:/x
      - .:/app
    tty: true
    stdin_open: true
`,
			warnings: []string{"flag_ignored --rm"},
		},
		{
			name: "long form flags over several lines",
			command: "docker container run \\\n  --name=app \\\n  --publish=127.0.0.1:53:53/udp \\\n  --hostname app.lan \\\n  --user 1000:1000 \\\n  --workdir /srv \\\n" +
				"  --label traefik.enable=true --label 'a b=c' \\\n  --cap-add NET_ADMIN --cap-drop ALL --device /dev/ttyUSB0 \\\n" +
				"  --security-opt seccomp=unconfined --tmpfs /run:rw,size=64m --sysctl net.core.somaxconn=1024 \\\n" +
				"  --add-host db:10.0.0.5 --dns 1.1.1.1 --group-add 20 --env-file .env --pid host --ipc host \\\n" +
				"  --init --read-only --privileged --platform linux/amd64 --pull always --stop-signal SIGINT --stop-timeout 30 \\\n" +
				"  --shm-size 256m --expose 9000 \\\n  alpine sleep 10",
			service: "app",
			compose: `services:
  app:
    image: alpine
    container_name: app
    command:
      - sleep
      - "10"
    ports:
      - "127.0.0.1:53:53/udp"
    expose:
      - "9000"
    tmpfs:
      - /run:rw,size=64m
    env_file:
      - .env
    labels:
      traefik.enable: "true"
      a b: c
    hostname: app.lan
    extra_hosts:
      - db:10.0.0.5
    dns:
      - 1.1.1.1
    working_dir: /srv
    user: "1000:1000"
    group_add:
      - "20"
    privileged: true
    cap_add:
      - NET_ADMIN
    cap_drop:
      - ALL
    devices:
      - /dev/ttyUSB0
    security_opt:
      - seccomp=unconfined
    sysctls:
      net.core.somaxconn: "1024"
    read_only: true
    shm_size: 256m
    init: true
    stop_signal: SIGINT
    stop_grace_period: 30s
    platform: linux/amd64
    pull_policy: always
    pid: host
    ipc: host
`,
		},
		{
			name:    "mounts, user network with alias, limits, health, logging",
			command: "docker run --net mynet --network-alias web --mount type=bind,source=/srv/a,target=/a,readonly --mount source=db,target=/var/lib/db --mount type=tmpfs,target=/t,tmpfs-size=64m -m 1g --cpus 1.5 --ulimit nofile=1024:2048 --ulimit core=-1 --health-cmd 'curl -f http://localhost' --health-interval 30s --health-timeout 5s --health-retries 3 --health-start-period 1m --log-driver json-file --log-opt max-size=10m --log-opt max-file=3 img:tag",
			service: "img",
			compose: `services:
  img:
    image: img:tag
    volumes:
      - type: bind
        source: /srv/a
        target: /a
        read_only: true
      - type: volume
        source: db
        target: /var/lib/db
      - type: tmpfs
        target: /t
        tmpfs:
          size: 64m
    networks:
      mynet:
        aliases:
          - web
    ulimits:
      nofile:
        soft: 1024
        hard: 2048
      core: -1
    mem_limit: 1g
    cpus: 1.5
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost"]
      interval: 30s
      timeout: 5s
      start_period: 1m
      retries: 3
    logging:
      driver: json-file
      options:
        max-size: 10m
        max-file: "3"
volumes:
  db: {}
networks:
  mynet:
    external: true
`,
		},
		{
			name:    "host network, entrypoint, no healthcheck, gpus",
			command: "docker run --network host --entrypoint '/bin/sh -c' --no-healthcheck --gpus all --privileged=false --init=true img",
			service: "img",
			compose: `services:
  img:
    image: img
    entrypoint: ["/bin/sh -c"]
    network_mode: host
    init: true
    healthcheck:
      disable: true
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
`,
		},
		{
			name:    "gpu devices",
			command: `docker run --gpus '"device=0,1",capabilities=utility' img`,
			service: "img",
			compose: `services:
  img:
    image: img
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              device_ids: ["0", "1"]
              capabilities: [gpu, utility]
`,
		},
		{
			name:    "flags without a conversion are reported and their values skipped",
			command: "docker run --cpuset-cpus 0-1 --ip 10.0.0.9 --link a:b -P --no-such-flag -q --label k=v nginx",
			service: "nginx",
			compose: `services:
  nginx:
    image: nginx
    labels:
      k: v
`,
			warnings: []string{
				"flag_unsupported --cpuset-cpus", "flag_unsupported --ip", "flag_unsupported --link",
				"flag_unsupported --publish-all", "flag_unsupported --no-such-flag", "flag_unsupported --quiet",
			},
		},
		{
			name:    "invalid values are reported",
			command: "docker run --restart sometimes --cpus abc -m 5x --stop-timeout soon --pull sometimes --health-interval 30 --ulimit nofile=x -p 80:80:80:80 --sysctl bad nginx",
			service: "nginx",
			compose: `services:
  nginx:
    image: nginx
    ports:
      - "80:80:80:80"
`,
			warnings: []string{
				"flag_value_invalid --restart", "flag_value_invalid --publish", "flag_value_invalid --sysctl", "flag_value_invalid --ulimit",
				"flag_value_invalid --memory", "flag_value_invalid --cpus", "flag_value_invalid --stop-timeout",
				"flag_value_invalid --health-interval", "flag_value_invalid --pull",
			},
		},
		{
			name:    "command substitution cannot be converted",
			command: "docker run -u $(id -u):$(id -g) nginx",
			service: "nginx",
			compose: `services:
  nginx:
    image: nginx
    user: $(id -u):$(id -g)
`,
			warnings: []string{"flag_value_invalid --user"},
		},
		{
			name:    "network alias without a user network",
			command: "docker run --network-alias web --restart no nginx",
			service: "nginx",
			compose: `services:
  nginx:
    image: nginx
    restart: "no"
`,
			warnings: []string{"flag_ignored --network-alias"},
		},
		{
			name:    "prompt and sudo are ignored, image digest dropped from the name",
			command: "$ sudo docker run ghcr.io/Org/My_App:1.0@sha256:abc -g 'daemon off;'",
			service: "my_app",
			compose: `services:
  my_app:
    image: ghcr.io/Org/My_App:1.0@sha256:abc
    command:
      - -g
      - daemon off;
`,
		},
		{
			name:    "service name from --name follows the stack name rules",
			command: "docker run --name My.App localhost:5000/team/web-ui:latest",
			service: "my-app",
			compose: `services:
  my-app:
    image: localhost:5000/team/web-ui:latest
    container_name: My.App
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ConvertDockerRun(tc.command)
			if err != nil {
				t.Fatalf("ConvertDockerRun: %v", err)
			}
			if res.Service != tc.service {
				t.Errorf("service = %q, want %q", res.Service, tc.service)
			}
			if res.Compose != tc.compose {
				t.Errorf("compose =\n%s\nwant\n%s", res.Compose, tc.compose)
			}
			got := []string{}
			for _, w := range res.Warnings {
				if w.Message == "" {
					t.Errorf("warning %+v has no message", w)
				}
				got = append(got, w.Code+" "+w.Flag)
			}
			want := tc.warnings
			if want == nil {
				want = []string{}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("warnings = %q, want %q", got, want)
			}
			// the output must be a compose file the linter can read, unless the command itself was wrong
			if !strings.Contains(strings.Join(got, ","), warnValueInvalid) {
				for _, d := range Lint(LintInput{Compose: res.Compose}, nil).Diagnostics {
					if d.Severity == "error" {
						t.Errorf("lint reports %+v for the converted file", d)
					}
				}
			}
			if match := regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`); !match.MatchString(res.Service) {
				t.Errorf("service name %q breaks the stack name rules", res.Service)
			}
		})
	}
}

func TestConvertDockerRunErrors(t *testing.T) {
	for name, command := range map[string]string{
		"empty":             "   ",
		"another command":   "docker ps -a",
		"not docker":        "podman run nginx",
		"docker container":  "docker container ls",
		"no image":          "docker run -d --name web",
		"flags only":        "docker run -it --rm",
		"unbalanced quote":  `docker run -e "A=1 nginx`,
		"unbalanced single": `docker run -e 'A=1 nginx`,
	} {
		t.Run(name, func(t *testing.T) {
			res, err := ConvertDockerRun(command)
			var coded *Error
			if !errors.As(err, &coded) || coded.Code != CodeConvertFailed || coded.Message == "" {
				t.Fatalf("ConvertDockerRun(%q) = %+v, %v; want an *Error with code %s", command, res, err, CodeConvertFailed)
			}
		})
	}
}

func TestConvertResultJSON(t *testing.T) {
	res, err := ConvertDockerRun("docker run nginx")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(res)
	if !strings.Contains(string(raw), `"warnings":[]`) {
		t.Fatalf("result JSON %s must carry an empty warnings list, not null", raw)
	}
}
