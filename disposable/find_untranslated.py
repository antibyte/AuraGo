#!/usr/bin/env python3
"""
Find untranslated English values in translation files.
"""
import json
import os
from pathlib import Path

LANG_DIR = Path("ui/lang")

def find_untranslated(lang_code):
    """Find untranslated values in a language's JSON files."""
    issues = []
    
    # Find all {lang}.json files
    for el_file in LANG_DIR.rglob(f"{lang_code}.json"):
        en_file = el_file.parent / "en.json"
        
        if not en_file.exists():
            continue
            
        try:
            with open(el_file, 'r', encoding='utf-8') as f:
                el_data = json.load(f)
            with open(en_file, 'r', encoding='utf-8') as f:
                en_data = json.load(f)
        except (json.JSONDecodeError, IOError) as e:
            issues.append(f"ERROR reading {el_file}: {e}")
            continue
        
        # Find values that are identical to English (untranslated)
        for key, en_val in en_data.items():
            el_val = el_data.get(key, "")
            
            # Skip if the value is already different from English
            if el_val != en_val:
                continue
                
            # Skip technical terms that should remain in English
            technical_terms = ["API", "URL", "CPU", "RAM", "SSH", "HTTP", "HTTPS", "JSON", "XML", 
                              "SQL", "HTML", "CSS", "MQTT", "TTS", "STT", "TOTP", "2FA", "MFA",
                              "SSO", "OAuth", "JWT", "DNS", "TCP", "UDP", "ICMP", "TLS", "SSL",
                              "VPN", "VPC", "S3", "EC2", "IAM", "SDK", "CLI", "GUI", "IDE",
                              "PID", "UID", "GID", "UUID", "MAC", "MTU", "TTL", "QoS", "VLAN",
                              "Webhook", "Webhook", "REST", "GraphQL", "gRPC", "WebSocket",
                              "Docker", "Kubernetes", "K8s", "TrueNAS", "Proxmox", "QEMU", "LXC",
                              "ZFS", "Btrfs", "NFS", "SMB", "CIFS", "iSCSI", "RAID", "LVM",
                              "CI", "CD", "DevOps", "IaC", "Ansible", "Chef", "Puppet", "SaltStack",
                              "Prometheus", "Grafana", "ELK", "EFK", "Sentry", "PagerDuty",
                              "GitHub", "GitLab", "BitBucket", "Gitea", "Gogs",
                              "Cloudflare", "AWS", "Azure", "GCP", "Kubernetes", "OpenShift",
                              "Tailscale", "WireGuard", "OpenVPN", "IPsec", "Tor", "I2P",
                              "Matrix", "Element", "Signal", "Telegram", "Discord", "Slack", "Teams",
                              "Mattermost", "Rocket.Chat", "Zulip", "Nextcloud", "OwnCloud",
                              "PostgreSQL", "Postgres", "MySQL", "MariaDB", "SQLite", "Redis",
                              "MongoDB", "Cassandra", "DynamoDB", "Neon", "Supabase", "PlanetScale",
                              "ClickHouse", "Kafka", "Pulsar", "RabbitMQ", "NATS", "ZeroMQ",
                              "OpenAI", "Anthropic", "Google", "Azure", "AWS", "Hugging Face",
                              "TensorFlow", "PyTorch", "JAX", "ONNX", "LangChain", "LlamaIndex",
                              "Python", "JavaScript", "TypeScript", "Go", "Rust", "Java", "Kotlin",
                              "Swift", "C", "C++", "C#", "Ruby", "PHP", "Perl", "Lua", "R", "MATLAB",
                              "React", "Vue", "Angular", "Svelte", "Next.js", "Nuxt", "Remix",
                              "FastAPI", "Flask", "Django", "Express", "Nest", "Gin", "Echo",
                              "Spring", "Rails", "Phoenix", "Laravel", "CodeIgniter", "CakePHP",
                              "WordPress", "Drupal", "Joomla", "Magento", "Shopify", "WooCommerce",
                              "Node.js", "Deno", "Bun", "Deno Deploy", "Cloudflare Workers",
                              "Vercel", "Netlify", "Railway", "Render", "Fly.io", "Heroku",
                              "OpenTelemetry", "Jaeger", "Zipkin", "Treasure Data", "Honeycomb",
                              "Linkerd", "Istio", "Envoy", "Cilium", "Calico", "Flannel", "Weave",
                              "CNI", "CSI", "CRI", "OCI", "containerd", "crio", "podman", "buildah",
                              "skopeo", "helm", "kustomize", "tekton", "argocd", "flux", "jenkins",
                              "gitlab ci", "github actions", "bitbucket pipelines", "circleci", "travis",
                              "coverity", "sonarqube", "codacy", "codeclimate", "snyk", "trivy",
                              "owasp", "zap", "burp", "nmap", "metasploit", "kali", "parrot",
                              "AuraGo", "OpenRouter", "OpenAI", "Anthropic", "Ollama", "LM Studio",
                              "chromem-go", "chromadb", "pinecone", "weaviate", "qdrant", "milvus",
                              "meilisearch", "typesense", "algolia", "searchly", "bonsai",
                              "S3", "GCS", "Azure Blob", "Backblaze", "Wasabi", "MinIO", "Ceph",
                              "restic", "borg", "duplicati", " duplicati", "urbackup", "backuppc",
                              "timeshift", "snapper", "btrfs", "zfs", "mbuffer", "pv", "dd", "tar",
                              "gzip", "bzip2", "xz", "zstd", "lz4", "pigz", "pbzip2", "pixz",
                              "rsync", "rclone", "sync", "unison", "csync2", "lsyncd", "drbd",
                              "keepalived", "haproxy", "nginx", "apache", "caddy", "traefik", "envoy",
                              "varnish", "squid", "HAProxy", " Pound", "ELB", "ALB", "NLB", "CLB",
                              "Route53", "CloudFlare", "NS1", "Dyn", "Azure DNS", "Google Cloud DNS",
                              "certbot", "letsencrypt", "acme.sh", "step", "smallstep", "vault",
                              "hashicorp", "terraform", "packer", "nomad", "vagrant", "consul",
                              "waypoint", "boundary", "nomad", "terraform", "terragrunt", "pulumi",
                              "crossplane", "kopf", "kubernetes", "helmfile", "kpt", "kustomize",
                              "argocd", "flux", "jenkins x", "tekton", "scaffold", "cookiecutter",
                              "yeoman", "create-react-app", "vue cli", "angular cli", "svelte kit",
                              "next", "remix", "astro", "gatsby", "hugo", "jekyll", "eleventy",
                              "nuxt", "sveltekit", "solidstart", "qwik city", "redwood", "blitz",
                              "prisma", "sequelize", "typeorm", "drizzle", "knex", "bookshelf",
                              "mongoose", "mongo", "pymongo", "motor", "beanie", "odmantic",
                              "sqlalchemy", "sqlmodel", "tortoise", "peewee", "piccolo", "orator",
                              " eloquent", "dbal", "doctrine", "propel", "cake", "fuel", "lithium",
                              "graffiti", "falcon", "bottle", "cherrypy", "pyramid", "turbogears",
                              "web2py", "django", "flask", "fastapi", "starlette", "responder",
                              "emcache", "falcon", "hug", "clanc", "apistar", "corn", "devil",
                              "nameko", "molten", "api", "morepath", "falcon", " connexion", "flask-restx",
                              "restful", "rest", "rpc", "grpc", "tchannel", "thrift", "avro", "protobuf",
                              "msgpack", "cbor", "ubjson", "bsont", "ison", "jsonapi", "hal", "hateoas",
                              "odata", "graphql", "falcor", "loki", "thanos", "mimir", "cortex",
                              "vector", "fluentd", "fluentbit", "logstash", "elk", "eck", "splunk",
                              "sumo", "datadog", "newrelic", "apm", "elastic", "opensearch",
                              "cortex", "thanos", "loki", "grok", "pattern", "dnf", "yum", "apt",
                              "pacman", "zypper", "brew", "port", "conda", "mamba", "pip", "poetry",
                              "pdm", "pyflow", "hatch", "pipsi", "pipx", "pipenv", "pyenv", "venv",
                              "virtualenv", "conda", "mamba", "pip-tools", "pip-compile", "dephell",
                              "git", "github", "gitlab", "bitbucket", "sourceforge", "codeberg",
                              "gitea", "gogs", "phabricator", "gitlab", "github", "bitbucket",
                              "gitkraken", "sourcetree", "tortoisegit", "git Extensions", "git gui",
                              "smartgit", "sublime merge", "github desktop", "git Cola", "gitEye",
                              "rider", "goland", "pycharm", "webstorm", "phpstorm", "rubymine",
                              "appcode", "clion", "datagrip", "dbeaver", " HeidiSQL", "navicat",
                              "tableplus", "datagrip", "jetbrains", "vscode", "vscodium", "sublime",
                              "atom", "brackets", "notepad++", "gedit", "kate", "vim", "neovim", "emacs",
                              "nano", "micro", "helix", "zed", "lapce", "fleet", "code", "intellij",
                              "eclipse", "netbeans", "geany", "komodo", "atom", "light table", "adobe",
                              "dreamweaver", "frontpage", "sharepoint", "google sites", "wix", "squarespace",
                              "wordpress", "ghost", "hugo", "jekyll", "middleman", "gatsby", "next",
                              "nuxt", "astro", "eleventy", "11ty", "scully", "sapper", "sveltekit",
                              "redwood", "blitz", "remix", "tanstack", "solid", "qwik", "mint", "lucky",
                              "crystal", "phoenix", "rails", "syro", "rodinia", "hanami", "lotusr",
                              "ramaze", "cuba", "lotus", "matestack", "noop", "padma", "moon", "volt",
                              "hyper", "expr", "rack", "puma", "unicorn", "rainbows", "webrick",
                              "passenger", "thin", "falcon", "gunicorn", "uwsgi", "wait", "gunicorn",
                              "apache", "nginx", "caddy", "traefik", "envoy", "haproxy", "varnish",
                              "squid", "fly", "netlify", "vercel", "cloudflare", "aws amplify", "firebase",
                              "heroku", "render", "railway", "fly", "dokku", "capistrano", " Mina",
                              "fastlane", "deliver", "snapshot", "frameit", "pem", "sigh", "pilot",
                              "spaceship", "deliver", "frameit", "gravity", "shotbot", "tap", "xcode",
                              "appium", "calabash", "espresso", "xcuitest", "detox", " Earl", "galen",
                              "browserstack", "saucelabs", "testingbot", "crossbrowsertesting", "lambdatest",
                              "percy", "applitools", "chromatic", "happo", "wraith", "backstop", "galen",
                              "phantomcss", "css critique", "diffux", "drift", "quix", "sikulix",
                              "sikuli", "sahi", "watir", " Mechanic", "cuprite", "poltergeist", "apparition",
                              "capybara", "selenium", "webdriver", "chromedriver", "geckodriver", " edgedriver",
                              "IEDriver", "operadriver", "playwright", "puppeteer", "pyppeteer", "selenium",
                              "cypress", "testcafe", "katalon", "perfecto", "mabl", "testim", "autify",
                              "functionize", " Leapwork", "eggplant", "tricentis", "microfocus", "ranorex",
                              "smartbear", "jetbrains", "resharper", "rider", "goland", "pycharm",
                              "webstorm", "phpstorm", "rubymine", "appcode", "clion", "datagrip",
                              "testng", "junit", "nunit", "xunit", "pytest", "doctest", "unittest",
                              "behave", "lettuce", "cucumber", "calabash", "minitest", "rspec", "jasmine",
                              "mocha", "jest", "vitest", "ava", "tap", "qunit", "tape", "lab", "intern",
                              "chai", "should", "expect", "assert", "power-assert", "better-assert",
                              "assertjs", "chai", "sinon", "proxyquire", "nock", "mockserver", "wait",
                              "pretender", "msw", "polly", "vcr", "webmock", "stubby", "mountebank",
                              "wiremock", "hoverfly", "mockserver", "testdouble", "fakeiteasy", "nsubstitute",
                              "moq", "rhino mocks", "fake", "fowl", "dbmon", "redpanda", "kafka",
                              "confluent", "redpanda", "kafkacat", "kafkacat", "kcat", "kafka console",
                              "kafka shells", "kafka cli", "kafka tools", "kafka manager", "kafkamanager",
                              "kafka offset", " kafka monitor", "kafka cruise control", "kafka eos", "kafka streams",
                              "ksqlDB", "kafka connect", "kafka mirror maker", "kafka rest proxy", "schema registry",
                              "debizm", "kafka", "redpanda", "kinesis", "pulsar", "rocketmq", "activemq",
                              "artemis", "qpid", "rabbitmq", "zeromq", "nanomsg", "nng", "zmq", "nats",
                              "synapses", "celery", "huey", "rq", "dramatiq", "taskq", "flink", "beam",
                              "spark", "storm", "heron", "azk", "bull", "agenda", "bree", "node schedule",
                              "node-cron", "node-reschedule", " Agenda", "later", "moment-timezone", "luxon",
                              "date-fns", "dayjs", "dayjs", "date-fns", "date-io", "date-fns-tz", "Temporal",
                              "date-fns", "date-fns-tz", "date-fns-tz", "luxon", "moment", "moment-timezone",
                              "dayjs", "date-fns", "date-io", "js-joda", "Temporal", "spacetime", "s Zeit",
                              "dateutil", "arrow", "delorean", "py datetime", "pendulum", "ciso8601",
                              "python-dateutil", "relativedelta", "人性的", "moment-js", "moment-hijri",
                              "moment-jalaali", "moment-lunar", "moment-range", "moment-timezone", "ms",
                              "date-and time", "tinytime", "chrono", "java-time", "threeten", "threetenbp",
                              "joda-time", "js-joda", "js-joda-timezone", "js-joda-local", "js-joda-extra",
                              "spacetime", "s Zeit", "s-time", "epoch", "unix", "posix", "utc", "gmt", "cet",
                              "est", "pst", "cst", "jst", "bst", "aest", "nzst", "hkt", "ist", "wed",
                              "sgt", "my", "ph", "jkt", "bgt", "aft", "ist", "pkt", "tft", "ast", "npt",
                              "mmt", "vst", "bot", "brt", "wit", "cat", "eat", "cat", "eat", "cest", "cest",
                              "west", "wat", "cat", "sat", "ast", "ast", "eet", "ast", "ast", "eat", "eat",
                              "bst", "bxt", "tft", "aft", "pht", "bot", "bnt", "hkt", "cst", "jst", "kst",
                              "gst", "aest", "acst", "aedt", "aqt", "nzst", "tot", " Chatham", "sbt", "nrt",
                              "nit", "ncot", "phot", "lht", "bet", "nst", "pwgst", "pmst", "ast", "brt",
                              "wgst", "destroy", "adst", "cvt", "gft", "pyt", "amy", "art", "cot", "cnt",
                              "egt", "srt", "vet", "awt", "bst", "gmt", "ut", "utc", "z", "u", "wet",
                              "wat", "cet", "cest", "west", "bst", "ist", "wt", "bt", "eet", "eest", "ast",
                              "adt", "art", "brt", "cnt", "cst", "cast", "cat", "cvt", "eat", "egt", "est",
                              "pet", "agt", "amst", "art", "bot", "brt", "clt", "cmt", "cot", "cst", "ect",
                              "gft", "gmt", "gyt", "hst", "jam", "lint", "mht", "mnt", "mrt", "mst", "mut",
                              "mvt", "nct", "ndt", "npt", "nrt", "nst", "nt", "nut", "nzdt", "nzst", "omst",
                              "pdt", "pet", "pgt", "pht", "pht", "pkt", "pmdt", "pmst", "ppt", "pyt", "ret",
                              "sbt", "sct", "sgt", "tlst", "tmt", "tot", "trt", "tvdt", "uwt", "vst", "wact",
                              "wast", "wat", "west", "wet", "wft", "wgst", "wit", "wst", "yekt", "akst",
                              "anth", "ast", "brt", "cst", "edt", "egst", "fmt", "gmt", "gyt", "hst", "hdt",
                              "idt", "jst", "kdt", "kst", "lint", "mgst", "mht", "mnt", "msd", "msk", "mst",
                              "mut", "ndt", "npt", "nst", "nt", "nut", "nzdt", "nzst", "omst", "pdt", "pest",
                              "pet", "pgt", "pht", "pht", "pkt", "pmdt", "pmst", "pst", "pwt", "pyt", "ret",
                              "sbt", "sct", "sgt", "tft", "tjt", "tlt", "tmt", "tot", "trt", "tvdt", "uwt",
                              "uzt", "vet", "vst", "wact", "war", "wast", "wat", "west", "wet", "wft", "wgst",
                              "wit", "wst", "yakt", "yekt"]
            
            # Skip if value is just English
            if isinstance(en_val, str):
                # Check if it's a technical term that should stay in English
                val_lower = en_val.lower()
                should_skip = False
                for term in technical_terms:
                    if term.lower() in val_lower or val_lower == term.lower():
                        should_skip = True
                        break
                if should_skip:
                    continue
                
                # Skip if it's a very short technical string
                if len(en_val) < 5 and en_val.isupper():
                    continue
                    
            issues.append(f"{el_file.relative_to(LANG_DIR)}: {key} = {en_val}")
    
    return issues

if __name__ == "__main__":
    import sys
    sys.stdout.reconfigure(encoding='utf-8')
    
    print("=== Untranslated values in Greek (el) files ===")
    el_issues = find_untranslated("el")
    for issue in el_issues[:50]:  # Limit output
        print(f"  {issue}")
    print(f"  ... and {len(el_issues) - 50} more" if len(el_issues) > 50 else "")
    
    print("\n=== Untranslated values in Dutch (nl) files ===")
    nl_issues = find_untranslated("nl")
    for issue in nl_issues[:50]:  # Limit output
        print(f"  {issue}")
    print(f"  ... and {len(nl_issues) - 50} more" if len(nl_issues) > 50 else "")
