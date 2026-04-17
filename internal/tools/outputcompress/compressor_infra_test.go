package outputcompress

import (
	"fmt"
	"strings"
	"testing"
)

// ── V5: K8s Compressor Tests ──────────────────────────────────────────

func TestCompressK8sLogs(t *testing.T) {
	output := "2026-04-13T12:00:00Z [INFO] server started\n" +
		strings.Repeat("2026-04-13T12:00:01Z [INFO] request ok\n", 60) +
		"2026-04-13T12:01:00Z [ERROR] connection lost\n"
	result := compressK8sLogs(output)
	if !strings.Contains(result, "ERROR") {
		t.Error("should preserve error lines")
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression for k8s logs: %d >= %d", len(result), len(output))
	}
}

func TestCompressK8sGet_Small(t *testing.T) {
	output := "NAME       READY   STATUS    RESTARTS   AGE\nnginx      1/1     Running   0          1h\nredis      1/1     Running   0          2h"
	result := compressK8sGet(output)
	// Small output (<=8 lines) should pass through
	if !strings.Contains(result, "nginx") {
		t.Error("small k8s get should preserve content")
	}
}

func TestCompressK8sGet_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("NAME                       READY   STATUS      RESTARTS   AGE\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("app-%d-abc   1/1     Running   0   %dh\n", i, i))
	}
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("app-%d-def   0/1     Pending    0   %dm\n", i+20, i))
	}
	for i := 0; i < 3; i++ {
		sb.WriteString(fmt.Sprintf("app-%d-ghi   0/1     CrashLoopBackOff   5   %dh\n", i+30, i))
	}
	result := compressK8sGet(sb.String())
	if !strings.Contains(result, "Running") {
		t.Error("should contain Running count")
	}
	if !strings.Contains(result, "Pending") {
		t.Error("should contain Pending count")
	}
	if !strings.Contains(result, "Failed") {
		t.Error("should contain Failed count for CrashLoopBackOff")
	}
	if !strings.Contains(result, "CrashLoopBackOff") {
		t.Error("should include failed/pending lines for context")
	}
}

func TestCompressK8sDescribe(t *testing.T) {
	output := `Name:         nginx-deployment-abc123
Namespace:    default
Priority:     0
Node:         node-1/10.0.0.1
Labels:       app=nginx
Status:       Running
IP:           10.244.0.5
Containers:
	 nginx:
	   Image:          nginx:latest
	   Port:           80/TCP
Conditions:
	 Type           Status
	 Ready          True
	 PodScheduled   True
Events:
	 Type    Reason   Age   From       Message
	 Normal  Pulled   5m    kubelet    Successfully pulled image
	 Normal  Created  5m    kubelet    Created container
	 Warning Failed   1m    kubelet    Error: ImagePullBackOff
	 Warning BackOff  30s   kubelet    Back-off restarting failed container`
	result := compressK8sDescribe(output)
	if !strings.Contains(result, "Name:") {
		t.Error("should contain Name field")
	}
	if !strings.Contains(result, "Status:") {
		t.Error("should contain Status field")
	}
	if !strings.Contains(result, "Node:") {
		t.Error("should contain Node field")
	}
	if !strings.Contains(result, "Warning") {
		t.Error("should include warning events")
	}
	if !strings.Contains(result, "Ready") {
		t.Error("should include Conditions")
	}
}

func TestCompressK8s_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"kubectl logs pod-1", "k8s-logs"},
		{"kubectl get pods", "k8s-get"},
		{"kubectl describe pod nginx", "k8s-describe"},
		{"kubectl top nodes", "k8s-top"},
		{"kubectl apply -f.yaml", "k8s-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ── V5: Systemctl Compressor Tests ────────────────────────────────────

func TestCompressSystemctlStatus(t *testing.T) {
	output := `● nginx.service - A high performance web server
	    Loaded: loaded (/lib/systemd/system/nginx.service; enabled)
	    Active: active (running) since Mon 2026-04-13 12:00:00 UTC; 2h ago
	  Main PID: 1234 (nginx)
	     Tasks: 5 (limit: 4915)
	    Memory: 4.2M
	       CPU: 1.234s
	    CGroup: /system.slice/nginx.service
	            ├─1234 "nginx: master process"
	            └─1235 "nginx: worker process"

    Apr 13 12:00:00 server nginx[1234]: start processing
    Apr 13 12:00:01 server nginx[1234]: request handled
    Apr 13 12:00:02 server nginx[1234]: request handled
    Apr 13 12:30:00 server nginx[1234]: error: connection reset by peer
    Apr 13 12:30:01 server nginx[1234]: warning: slow upstream response
    Apr 13 13:00:00 server nginx[1234]: request handled
    Apr 13 13:00:01 server nginx[1234]: request handled
    Apr 13 13:00:02 server nginx[1234]: request handled
    Apr 13 13:00:03 server nginx[1234]: request handled
    Apr 13 13:00:04 server nginx[1234]: request handled`
	result := compressSystemctlStatus(output)
	if !strings.Contains(result, "Active:") {
		t.Error("should contain Active field")
	}
	if !strings.Contains(result, "Main PID:") {
		t.Error("should contain Main PID field")
	}
	if !strings.Contains(result, "Memory:") {
		t.Error("should contain Memory field")
	}
	if !strings.Contains(result, "error") {
		t.Error("should include error log lines")
	}
	if !strings.Contains(result, "warning") {
		t.Error("should include warning log lines")
	}
}

func TestCompressSystemctlList(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("  UNIT                           LOAD   ACTIVE   SUB          DESCRIPTION\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("  service-%d.service              loaded active   running      Service %d\n", i, i))
	}
	sb.WriteString("  broken.service                 loaded failed   failed       Broken Service\n")
	result := compressSystemctlList(sb.String())
	if !strings.Contains(result, "Running") {
		t.Error("should contain Running count")
	}
	if !strings.Contains(result, "Failed") {
		t.Error("should contain Failed count")
	}
	if !strings.Contains(result, "broken.service") {
		t.Error("should include failed unit lines")
	}
}

func TestCompressSystemctl_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"systemctl status nginx", "systemctl-status"},
		{"systemctl list-units", "systemctl-list"},
		{"systemctl list-unit-files", "systemctl-list"},
		{"systemctl restart nginx", "systemctl-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ── V5: Journalctl/Logs Routing Tests ─────────────────────────────────

func TestCompressLogs_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"journalctl -u nginx", "logs"},
		{"logcli query '{app=\"nginx\"}'", "logs"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ── V6: Docker Compose Tests ──────────────────────────────────────────

func TestCompressComposePs_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("NAME                IMAGE          COMMAND   SERVICE   STATUS          PORTS\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("app-%d              nginx:latest   \"/bin/sh\" app-%d     Up 2 hours      0.0.0.0:808%d->80/tcp\n", i, i, i%10))
	}
	sb.WriteString("app-broken          redis:7        \"redis…\"  cache     Exited (1) 5m\n")
	result := compressComposePs(sb.String())
	if !strings.Contains(result, "Running") {
		t.Error("should contain Running count")
	}
	if !strings.Contains(result, "Stopped") {
		t.Error("should contain Stopped count")
	}
	if !strings.Contains(result, "app-broken") {
		t.Error("should include stopped services")
	}
}

func TestCompressComposeConfig_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("services:\n")
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("  service-%d:\n    image: app:%d\n    ports:\n      - \"808%d:80\"\n", i, i, i))
	}
	sb.WriteString("networks:\n  default:\n    driver: bridge\n")
	sb.WriteString("volumes:\n  data:\n")
	result := compressComposeConfig(sb.String())
	if !strings.Contains(result, "services") {
		t.Error("should mention services")
	}
	if !strings.Contains(result, "30 services") {
		t.Error("should count services")
	}
}

func TestCompressDockerCompose_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"docker compose ps", "compose-ps"},
		{"docker compose logs", "compose-logs"},
		{"docker compose config", "compose-config"},
		{"docker compose events", "compose-events"},
		{"docker compose up -d", "compose-generic"},
		{"docker-compose ps", "compose-ps"},
		{"docker-compose logs -f", "compose-logs"},
		{"docker_compose ps", "compose-ps"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ── V6: Helm Tests ────────────────────────────────────────────────────

func TestCompressHelmList_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("NAME            NAMESPACE       REVISION        STATUS          CHART                   APP VERSION\n")
	for i := 0; i < 20; i++ {
		sb.WriteString(fmt.Sprintf("app-%d           default         %d              deployed        chart-%d-1.0.%d        1.0.%d\n", i, i+1, i, i, i))
	}
	sb.WriteString("app-broken      default         3               failed          broken-1.0.0            1.0.0\n")
	result := compressHelmList(sb.String())
	if !strings.Contains(result, "Deployed") {
		t.Error("should contain Deployed count")
	}
	if !strings.Contains(result, "Failed") {
		t.Error("should contain Failed count")
	}
	if !strings.Contains(result, "app-broken") {
		t.Error("should include failed releases")
	}
}

func TestCompressHelmStatus(t *testing.T) {
	output := `STATUS: deployed
REVISION: 5
CHART: nginx-ingress-4.0.1
NAMESPACE: ingress-nginx
LAST DEPLOYED: Mon Apr 13 12:00:00 2026
NOTES:
The nginx ingress controller has been installed.

==> v1/Service
NAME                          TYPE          CLUSTER-IP     EXTERNAL-IP   PORT(S)
nginx-ingress-controller      LoadBalancer  10.0.0.1       pending       80:31234/TCP,443:31235/TCP
READY   REASON
`
	result := compressHelmStatus(output)
	if !strings.Contains(result, "STATUS:") {
		t.Error("should contain STATUS field")
	}
	if !strings.Contains(result, "REVISION:") {
		t.Error("should contain REVISION field")
	}
}

func TestCompressHelm_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"helm list", "helm-list"},
		{"helm ls", "helm-list"},
		{"helm status nginx", "helm-status"},
		{"helm history nginx", "helm-history"},
		{"helm get values nginx", "helm-get"},
		{"helm repo update", "helm-repo"},
		{"helm install nginx bitnami/nginx", "helm-generic"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ── V6: Terraform Tests ───────────────────────────────────────────────

func TestCompressTerraformPlan(t *testing.T) {
	output := `Terraform will perform the following actions:

  # aws_instance.web will be created
  + resource "aws_instance" "web" {
      + ami           = "ami-12345"
      + instance_type = "t3.micro"
    }

  # aws_security_group.sg will be destroyed
  - resource "aws_security_group" "sg" {
      - name = "old-sg"
    }

  # aws_db_instance.db will be updated in-place
  ~ resource "aws_db_instance" "db" {
      ~ instance_class = "db.t3.small" -> "db.t3.medium"
    }

Plan: 1 to add, 1 to change, 1 to destroy.`
	result := compressTerraformPlan(output)
	if !strings.Contains(result, "Plan:") {
		t.Error("should contain Plan summary")
	}
	if !strings.Contains(result, "will be created") {
		t.Error("should contain creation notice")
	}
	if !strings.Contains(result, "will be destroyed") {
		t.Error("should contain destruction notice")
	}
}

func TestCompressTerraformApply(t *testing.T) {
	output := `aws_instance.web: Creating...
aws_instance.web: Still creating... [10s elapsed]
aws_instance.web: Still creating... [20s elapsed]
aws_instance.web: Creation complete after 30s [id=i-12345]

Apply complete! Resources: 1 added, 0 changed, 0 destroyed.

Outputs:

  instance_ip = "10.0.0.1"
  instance_id = "i-12345"`
	result := compressTerraformApply(output)
	if !strings.Contains(result, "Apply complete!") {
		t.Error("should contain Apply complete")
	}
	if !strings.Contains(result, "instance_ip") {
		t.Error("should contain outputs")
	}
}

func TestCompressTerraformStateList(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("aws_instance.web-%d\n", i))
	}
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("aws_security_group.sg-%d\n", i))
	}
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("module.networking.aws_vpc.main-%d\n", i))
	}
	result := compressTerraformStateList(sb.String())
	if !strings.Contains(result, "resources") {
		t.Error("should contain resource count")
	}
	if !strings.Contains(result, "aws_instance") {
		t.Error("should group by resource type")
	}
}

func TestCompressTerraform_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"terraform plan", "tf-plan"},
		{"terraform apply", "tf-apply"},
		{"terraform show", "tf-show"},
		{"terraform state list", "tf-state"},
		{"terraform output", "tf-output"},
		{"terraform init", "tf-init"},
		{"terraform validate", "tf-generic"},
		{"tf plan", "tf-plan"},
		{"tf apply -auto-approve", "tf-apply"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ── V6: SSH Diagnostic Tests ──────────────────────────────────────────

func TestCompressDiskFree_HighUsage(t *testing.T) {
	output := `Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1       100G   20G   80G  20% /
/dev/sda2       500G  430G   70G  86% /data
/dev/sdb1       200G   10G  190G   5% /backup
tmpfs            16G   12G    4G  75% /dev/shm`
	result := compressDiskFree(output)
	if !strings.Contains(result, "86%") {
		t.Error("should include high-usage filesystem")
	}
	if strings.Contains(result, "/backup") {
		t.Error("should not include low-usage filesystem")
	}
}

func TestCompressDiskFree_AllLow(t *testing.T) {
	output := `Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1       100G   20G   80G  20% /
/dev/sdb1       200G   10G  190G   5% /backup`
	result := compressDiskFree(output)
	if !strings.Contains(result, "below 80%") {
		t.Error("should report all below threshold")
	}
}

func TestCompressDiskUsage_Large(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("%dM\t/path/dir-%d\n", 1000-i*20, i))
	}
	result := compressDiskUsage(sb.String())
	if !strings.Contains(result, "more entries") {
		t.Error("should truncate large output")
	}
}

func TestCompressProcessList_Large(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("PID   USER     %CPU  %MEM  COMMAND\n")
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf("%d    user     %d.%d   %d.%d   process-%d\n", 1000+i, i%10, i%5, i%8, i%3, i))
	}
	sb.WriteString("9999  user     95.2  80.1   runaway-process\n")
	result := compressProcessList(sb.String())
	if !strings.Contains(result, "runaway-process") {
		t.Error("should include high-resource process")
	}
	if !strings.Contains(result, "total processes") {
		t.Error("should show total count")
	}
}

func TestCompressNetworkConnections(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("State      Recv-Q Send-Q  Local Address:Port  Peer Address:Port\n")
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("LISTEN     0      128     0.0.0.0:%d         0.0.0.0:*\n", 8000+i))
	}
	for i := 0; i < 30; i++ {
		sb.WriteString(fmt.Sprintf("ESTAB      0      0       10.0.0.1:%d      10.0.0.2:%d\n", 40000+i, 80))
	}
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf("TIME-WAIT  0      0       10.0.0.1:%d      10.0.0.3:%d\n", 50000+i, 443))
	}
	result := compressNetworkConnections(sb.String())
	if !strings.Contains(result, "LISTEN") {
		t.Error("should contain LISTEN count")
	}
	if !strings.Contains(result, "ESTABLISHED") {
		t.Error("should contain ESTABLISHED count")
	}
	if !strings.Contains(result, "TIME-WAIT") {
		t.Error("should contain TIME-WAIT count")
	}
}

func TestCompressIpAddr(t *testing.T) {
	output := `1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 state UNKNOWN
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
    inet6 ::1/128 scope host
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 state UP
    link/ether 02:42:ac:11:00:02 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.2/16 brd 172.17.255.255 scope global eth0
    inet6 fe80::42:acff:fe11:2/64 scope link
3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 state DOWN
    link/ether 02:42:3a:5f:12:34 brd ff:ff:ff:ff:ff:ff
    inet 172.18.0.1/16 brd 172.18.255.255 scope global docker0`
	result := compressIpAddr(output)
	if !strings.Contains(result, "eth0") {
		t.Error("should contain interface name")
	}
	if !strings.Contains(result, "172.17.0.2") {
		t.Error("should contain IP address")
	}
	if !strings.Contains(result, "docker0") {
		t.Error("should contain all interfaces")
	}
}

func TestCompressIpRoute(t *testing.T) {
	output := `default via 10.0.0.1 dev eth0
10.0.0.0/24 dev eth0 proto kernel scope link src 10.0.0.5
172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1
172.18.0.0/16 dev br-1 proto kernel scope link src 172.18.0.1
192.168.1.0/24 dev wlan0 proto kernel scope link src 192.168.1.100`
	result := compressIpRoute(output)
	if !strings.Contains(result, "default") {
		t.Error("should contain default route")
	}
	if !strings.Contains(result, "other routes") {
		t.Error("should mention other routes")
	}
}

func TestCompressSSHDiag_Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"df -h", "df"},
		{"du -sh /*", "du"},
		{"ps aux", "ps"},
		{"ss -tulnp", "netstat"},
		{"netstat -tulnp", "netstat"},
		{"ip addr show", "ip-addr"},
		{"ip a", "ip-addr"},
		{"ip route show", "ip-route"},
		{"ip r", "ip-route"},
		{"ip link show", "ip-generic"},
		{"free -h", "free"},
		{"uptime", "uptime"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
// ─── V8: Network Diagnostics Compressor Tests ────────────────────────────────

func TestCompressPing_Success(t *testing.T) {
	output := `PING google.com (142.250.80.46): 56 data bytes
64 bytes from 142.250.80.46: icmp_seq=0 ttl=116 time=5.123 ms
64 bytes from 142.250.80.46: icmp_seq=1 ttl=116 time=4.987 ms
64 bytes from 142.250.80.46: icmp_seq=2 ttl=116 time=5.456 ms
64 bytes from 142.250.80.46: icmp_seq=3 ttl=116 time=5.012 ms
64 bytes from 142.250.80.46: icmp_seq=4 ttl=116 time=4.876 ms
--- google.com ping statistics ---
5 packets transmitted, 5 received, 0% packet loss
round-trip min/avg/max = 4.876/5.090/5.456 ms`

	result := compressPing(output)

	if !strings.Contains(result, "PING google.com") {
		t.Error("expected PING header")
	}
	if !strings.Contains(result, "5 packets transmitted, 5 received, 0% packet loss") {
		t.Error("expected packet statistics")
	}
	if !strings.Contains(result, "round-trip") {
		t.Error("expected RTT summary")
	}
	if strings.Contains(result, "icmp_seq=1") {
		t.Error("individual ICMP lines should be removed")
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(output))
	}
}

func TestCompressPing_Timeout(t *testing.T) {
	output := `PING unreachable.local (10.0.0.99): 56 data bytes
Request timeout for icmp_seq 0
Request timeout for icmp_seq 1
Request timeout for icmp_seq 2
--- unreachable.local ping statistics ---
3 packets transmitted, 0 received, 100% packet loss`

	result := compressPing(output)

	if !strings.Contains(result, "100% packet loss") {
		t.Error("expected packet loss in output")
	}
	if !strings.Contains(result, "Request timeout") {
		t.Error("expected timeout error lines preserved")
	}
}

func TestCompressPing_Unreachable(t *testing.T) {
	output := `PING badhost (0.0.0.0): 56 data bytes
ping: badhost: Name or service not known`

	result := compressPing(output)

	if !strings.Contains(result, "Name or service not known") {
		t.Error("expected DNS error preserved")
	}
}

func TestCompressPing_ShortOutput(t *testing.T) {
	output := "PING host (1.2.3.4): 56 data bytes\n64 bytes from 1.2.3.4: icmp_seq=0 ttl=64 time=1.0 ms\n"
	result := compressPing(output)

	// Short output (<=4 lines) should be preserved
	if result != output {
		t.Errorf("expected short output preserved, got: %s", result)
	}
}

func TestCompressDig_Large(t *testing.T) {
	output := `; <<>> DiG 9.18.0 <<>> example.com ANY
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12345
;; flags: qr rd ra; QUERY: 1, ANSWER: 5, AUTHORITY: 4, ADDITIONAL: 3

;; QUESTION SECTION:
;example.com.			IN	ANY

;; ANSWER SECTION:
example.com.		3600	IN	A	93.184.216.34
example.com.		3600	IN	A	93.184.216.35
example.com.		3600	IN	A	93.184.216.36
example.com.		3600	IN	AAAA	2606:2800:220:1:248:1893:25c8:1946
example.com.		3600	IN	MX	10 mail.example.com.

;; AUTHORITY SECTION:
example.com.		86400	IN	NS	a.iana-servers.net.
example.com.		86400	IN	NS	b.iana-servers.net.
example.com.		86400	IN	NS	c.iana-servers.net.
example.com.		86400	IN	NS	d.iana-servers.net.

;; ADDITIONAL SECTION:
mail.example.com.	3600	IN	A	10.0.0.1
mail.example.com.	3600	IN	A	10.0.0.2
mail.example.com.	3600	IN	AAAA	::1

;; Query time: 42 msec
;; SERVER: 8.8.8.8#53(8.8.8.8)
;; MSG SIZE  rcvd: 256`

	result := compressDig(output)

	if !strings.Contains(result, "QUESTION SECTION") {
		t.Error("expected question section")
	}
	if !strings.Contains(result, "ANSWER SECTION") {
		t.Error("expected answer section")
	}
	if !strings.Contains(result, "example.com.		3600	IN	A	93.184.216.34") {
		t.Error("expected answer records")
	}
	if !strings.Contains(result, "Query time") {
		t.Error("expected query time")
	}
	// Authority section should be removed
	if strings.Contains(result, "a.iana-servers.net") {
		t.Error("authority section should be removed")
	}
	// Additional section should be removed
	if strings.Contains(result, "mail.example.com.	3600	IN	A") {
		t.Error("additional section should be removed")
	}
	if len(result) >= len(output) {
		t.Errorf("expected compression: got %d >= %d", len(result), len(output))
	}
}

func TestCompressDig_NXDOMAIN(t *testing.T) {
	output := `; <<>> DiG 9.18.0 <<>> nonexistent.example.com
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NXDOMAIN, id: 54321

;; QUESTION SECTION:
;nonexistent.example.com.	IN	A

;; AUTHORITY SECTION:
example.com.		3600	IN	SOA	ns1.example.com. admin.example.com. 2024010101 3600 900 604800 86400

;; Query time: 15 msec
;; SERVER: 8.8.8.8#53(8.8.8.8)`

	result := compressDig(output)

	if !strings.Contains(result, "NXDOMAIN") {
		t.Error("expected NXDOMAIN status preserved")
	}
}

func TestCompressDig_ShortOutput(t *testing.T) {
	output := `; <<>> DiG 9.18.0 <<>> example.com
;; Answer: 93.184.216.34`
	result := compressDig(output)

	// Short output (<=10 lines) should be preserved
	if result != output {
		t.Errorf("expected short output preserved")
	}
}

func TestCompressDNS_Nslookup(t *testing.T) {
	output := "Server:\t\t8.8.8.8\nAddress:\t8.8.8.8#53\n\nNon-authoritative answer:\nName:\tgoogle.com\nAddress: 142.250.80.46\nName:\tgoogle.com\nAddress: 2606:2800:220:1:248:1893:25c8:1946\n"

	result := compressDNS(output)

	if !strings.Contains(result, "Server:") {
		t.Errorf("expected server info, got: %s", result)
	}
	if !strings.Contains(result, "google.com") {
		t.Error("expected answer")
	}
}

func TestCompressDNS_ShortOutput(t *testing.T) {
	output := "google.com has address 142.250.80.46\n"
	result := compressDNS(output)

	// Short output (<=5 lines) should be preserved
	if result != output {
		t.Errorf("expected short output preserved")
	}
}

func TestCompressCurl_JSON(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("{\n")
	for i := 0; i < 40; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": null,`, i) + "\n")
	}
	for i := 40; i < 60; i++ {
		sb.WriteString(fmt.Sprintf(`  "field_%d": "value_%d",`, i, i) + "\n")
	}
	sb.WriteString("}")

	result := compressCurl(sb.String())

	// Should apply JSON compaction
	if !strings.Contains(result, "omitted") {
		t.Error("expected null field omission")
	}
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression for JSON curl output")
	}
}

func TestCompressCurl_HTML(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<title>Test Page</title>\n</head>\n<body>\n")
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("<p>Paragraph %d with some content</p>\n", i))
	}
	sb.WriteString("</body>\n</html>")

	result := compressCurl(sb.String())

	if !strings.Contains(result, "Test Page") {
		t.Error("expected title preserved")
	}
	if !strings.Contains(result, "HTML response") {
		t.Error("expected HTML response note")
	}
	if len(result) >= len(sb.String()) {
		t.Errorf("expected compression for HTML curl output")
	}
}

func TestCompressCurl_Verbose(t *testing.T) {
	output := "> GET /api/health HTTP/2\n> Host: example.com\n> User-Agent: curl/8.0\n> Accept: */*\n>\n< HTTP/2 200\n< content-type: application/json\n< date: Mon, 15 Jan 2024 10:30:00 GMT\n< server: nginx\n< x-request-id: abc123\n< x-cache: HIT\n< content-length: 42\n<\n{\"status\":\"ok\",\"uptime\":123456,\"version\":\"1.0.0\"}"

	result := compressCurl(output)

	if !strings.Contains(result, "HTTP/2 200") {
		t.Errorf("expected HTTP status, got: %s", result)
	}
	if !strings.Contains(result, "content-type:") {
		t.Error("expected content-type header")
	}
	if !strings.Contains(result, "status") {
		t.Error("expected body preserved")
	}
}

func TestCompressCurl_PlainText(t *testing.T) {
	output := "Hello, this is a plain text response from a server.\nNothing special here.\n"
	result := compressCurl(output)

	// Plain text should go through generic compression
	if !strings.Contains(result, "Hello") {
		t.Error("expected content preserved")
	}
}

func TestCompressShell_V8Routing(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"ping google.com", "ping"},
		{"ping6 ipv6.google.com", "ping"},
		{"dig example.com", "dig"},
		{"nslookup google.com", "nslookup"},
		{"host google.com", "host"},
		{"curl http://example.com", "curl"},
		{"wget http://example.com", "curl"},
	}
	for _, tt := range tests {
		_, filter := compressShellOutput(tt.command, strings.Repeat("line\n", 50))
		if filter != tt.want {
			t.Errorf("compressShellOutput(%q) filter = %q, want %q", tt.command, filter, tt.want)
		}
	}
}
