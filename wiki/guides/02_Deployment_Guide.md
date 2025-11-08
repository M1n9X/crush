# Deployment Guide

## 🎯 Overview

This guide covers various deployment strategies for Crush, from individual developer setups to enterprise-wide deployments. Whether you're setting up Crush for a single developer, a team, or an entire organization, this guide provides the necessary configurations and best practices.

## 🏗️ Deployment Architectures

### 1. Individual Developer Setup

**Use Case**: Single developer using Crush on their local machine.

**Architecture**:
```
┌─────────────────┐
│  Developer      │
│  Workstation    │
│                 │
│  ┌───────────┐  │
│  │   Crush   │  │
│  │  (Local)  │  │
│  └───────────┘  │
│        │        │
└────────┼────────┘
         │
    ┌────▼────┐
    │   AI    │
    │ Providers│
    └─────────┘
```

**Configuration**:
```json
{
  "providers": {
    "openai": {
      "api_key": "$OPENAI_API_KEY"
    }
  },
  "options": {
    "data_directory": "~/.crush",
    "debug": false
  }
}
```

### 2. Team Shared Configuration

**Use Case**: Development team with shared configuration and standards.

**Architecture**:
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Developer 1   │    │   Developer 2   │    │   Developer N   │
│                 │    │                 │    │                 │
│  ┌───────────┐  │    │  ┌───────────┐  │    │  ┌───────────┐  │
│  │   Crush   │  │    │  │   Crush   │  │    │  │   Crush   │  │
│  └───────────┘  │    │  └───────────┘  │    │  └───────────┘  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │    Shared Config        │
                    │   (Git Repository)      │
                    └─────────────────────────┘
                                 │
                            ┌────▼────┐
                            │   AI    │
                            │ Providers│
                            └─────────┘
```

**Setup**:
```bash
# Create shared configuration repository
git init crush-config
cd crush-config

# Create team configuration
cat > crush.json << 'EOF'
{
  "$schema": "https://charm.land/crush.json",
  "providers": {
    "openai": {
      "api_key": "$OPENAI_API_KEY"
    },
    "anthropic": {
      "api_key": "$ANTHROPIC_API_KEY"
    }
  },
  "lsp": {
    "go": {
      "command": "gopls"
    },
    "typescript": {
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  },
  "options": {
    "context_paths": [
      ".cursorrules",
      "CRUSH.md",
      "README.md"
    ]
  },
  "permissions": {
    "allowed_tools": [
      "view",
      "edit",
      "ls",
      "grep",
      "git_status"
    ]
  }
}
EOF

# Add team-specific ignore patterns
cat > .crushignore << 'EOF'
# Build artifacts
dist/
build/
target/

# Dependencies
node_modules/
vendor/

# Sensitive files
.env
.env.local
secrets/
EOF

# Commit and push
git add .
git commit -m "Initial team Crush configuration"
git push origin main
```

**Team Member Setup**:
```bash
# Clone team configuration
git clone https://github.com/yourteam/crush-config.git ~/.config/crush

# Link configuration
ln -s ~/.config/crush/crush.json ~/.config/crush/crush.json

# Set up environment variables
echo "export OPENAI_API_KEY=your-key" >> ~/.bashrc
echo "export ANTHROPIC_API_KEY=your-key" >> ~/.bashrc
```

### 3. Enterprise Deployment

**Use Case**: Large organization with centralized management, compliance requirements, and multiple teams.

**Architecture**:
```
┌─────────────────────────────────────────────────────────────┐
│                    Enterprise Network                       │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │   Team A    │  │   Team B    │  │   Team C    │        │
│  │             │  │             │  │             │        │
│  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │        │
│  │ │ Crush   │ │  │ │ Crush   │ │  │ │ Crush   │ │        │
│  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
│           │                │                │              │
│           └────────────────┼────────────────┘              │
│                            │                               │
│  ┌─────────────────────────▼─────────────────────────┐    │
│  │            Configuration Management                │    │
│  │                                                    │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌───────────┐ │    │
│  │  │   Config    │  │   Secrets   │  │  Policies │ │    │
│  │  │  Service    │  │   Manager   │  │  Engine   │ │    │
│  │  └─────────────┘  └─────────────┘  └───────────┘ │    │
│  └────────────────────────────────────────────────────┘    │
│                            │                               │
│  ┌─────────────────────────▼─────────────────────────┐    │
│  │              Monitoring & Logging                  │    │
│  │                                                    │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌───────────┐ │    │
│  │  │   Metrics   │  │    Logs     │  │   Audit   │ │    │
│  │  │ Collection  │  │ Aggregation │  │   Trail   │ │    │
│  │  └─────────────┘  └─────────────┘  └───────────┘ │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                                │
                       ┌────────▼────────┐
                       │   AI Providers  │
                       │   (External)    │
                       └─────────────────┘
```

## 🐳 Containerized Deployment

### 1. Docker Setup

**Dockerfile**:
```dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o crush .

FROM alpine:latest

# Install dependencies
RUN apk --no-cache add ca-certificates sqlite git

WORKDIR /root/

# Copy binary
COPY --from=builder /app/crush .

# Create data directory
RUN mkdir -p /data/.crush

# Set environment variables
ENV CRUSH_DATA_DIR=/data/.crush
ENV CRUSH_CONFIG_PATH=/config/crush.json

# Expose volume for data persistence
VOLUME ["/data", "/config"]

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD crush --version || exit 1

ENTRYPOINT ["./crush"]
```

**Docker Compose**:
```yaml
version: '3.8'

services:
  crush:
    build: .
    container_name: crush
    volumes:
      - ./data:/data
      - ./config:/config
      - ./workspace:/workspace
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - CRUSH_DATA_DIR=/data/.crush
      - CRUSH_CONFIG_PATH=/config/crush.json
    working_dir: /workspace
    stdin_open: true
    tty: true
    restart: unless-stopped
    
    # Resource limits
    deploy:
      resources:
        limits:
          memory: 2G
          cpus: '1.0'
        reservations:
          memory: 512M
          cpus: '0.5'

  # Optional: Configuration management service
  config-manager:
    image: nginx:alpine
    container_name: crush-config
    volumes:
      - ./config:/usr/share/nginx/html:ro
    ports:
      - "8080:80"
    restart: unless-stopped
```

**Usage**:
```bash
# Build and run
docker-compose up -d

# Access Crush
docker-compose exec crush crush

# View logs
docker-compose logs -f crush

# Update configuration
docker-compose restart crush
```

### 2. Kubernetes Deployment

**Namespace**:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: crush
  labels:
    name: crush
```

**ConfigMap**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: crush-config
  namespace: crush
data:
  crush.json: |
    {
      "$schema": "https://charm.land/crush.json",
      "providers": {
        "openai": {
          "api_key": "$OPENAI_API_KEY"
        }
      },
      "options": {
        "data_directory": "/data/.crush",
        "debug": false
      }
    }
```

**Secret**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: crush-secrets
  namespace: crush
type: Opaque
data:
  openai-api-key: <base64-encoded-key>
  anthropic-api-key: <base64-encoded-key>
```

**Deployment**:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: crush
  namespace: crush
spec:
  replicas: 1
  selector:
    matchLabels:
      app: crush
  template:
    metadata:
      labels:
        app: crush
    spec:
      containers:
      - name: crush
        image: crush:latest
        imagePullPolicy: Always
        
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: crush-secrets
              key: openai-api-key
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: crush-secrets
              key: anthropic-api-key
        - name: CRUSH_CONFIG_PATH
          value: "/config/crush.json"
        
        volumeMounts:
        - name: config
          mountPath: /config
        - name: data
          mountPath: /data
        - name: workspace
          mountPath: /workspace
        
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        
        livenessProbe:
          exec:
            command:
            - crush
            - --version
          initialDelaySeconds: 30
          periodSeconds: 30
        
        readinessProbe:
          exec:
            command:
            - crush
            - --version
          initialDelaySeconds: 5
          periodSeconds: 10
      
      volumes:
      - name: config
        configMap:
          name: crush-config
      - name: data
        persistentVolumeClaim:
          claimName: crush-data
      - name: workspace
        persistentVolumeClaim:
          claimName: crush-workspace
```

**Persistent Volume Claims**:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: crush-data
  namespace: crush
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: crush-workspace
  namespace: crush
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 50Gi
```

## ☁️ Cloud Deployment

### 1. AWS Deployment

**EC2 Instance Setup**:
```bash
#!/bin/bash
# User data script for EC2 instance

# Update system
yum update -y

# Install dependencies
yum install -y git docker

# Start Docker
systemctl start docker
systemctl enable docker

# Add ec2-user to docker group
usermod -a -G docker ec2-user

# Install Docker Compose
curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose

# Clone deployment configuration
git clone https://github.com/yourorg/crush-deployment.git /opt/crush

# Set up environment
cd /opt/crush
cp .env.example .env

# Start services
docker-compose up -d

# Set up log rotation
cat > /etc/logrotate.d/crush << 'EOF'
/opt/crush/logs/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 root root
}
EOF
```

**CloudFormation Template**:
```yaml
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Crush AI Assistant Deployment'

Parameters:
  InstanceType:
    Type: String
    Default: t3.medium
    AllowedValues: [t3.small, t3.medium, t3.large]
  
  KeyName:
    Type: AWS::EC2::KeyPair::KeyName
    Description: EC2 Key Pair for SSH access

Resources:
  CrushSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: Security group for Crush instance
      SecurityGroupIngress:
        - IpProtocol: tcp
          FromPort: 22
          ToPort: 22
          CidrIp: 0.0.0.0/0
        - IpProtocol: tcp
          FromPort: 8080
          ToPort: 8080
          CidrIp: 0.0.0.0/0

  CrushInstance:
    Type: AWS::EC2::Instance
    Properties:
      ImageId: ami-0abcdef1234567890  # Amazon Linux 2
      InstanceType: !Ref InstanceType
      KeyName: !Ref KeyName
      SecurityGroups:
        - !Ref CrushSecurityGroup
      UserData:
        Fn::Base64: !Sub |
          #!/bin/bash
          # User data script here
      Tags:
        - Key: Name
          Value: Crush-AI-Assistant

Outputs:
  InstanceId:
    Description: Instance ID
    Value: !Ref CrushInstance
  
  PublicIP:
    Description: Public IP address
    Value: !GetAtt CrushInstance.PublicIp
```

### 2. Google Cloud Platform

**Compute Engine Setup**:
```bash
# Create instance
gcloud compute instances create crush-instance \
    --zone=us-central1-a \
    --machine-type=e2-medium \
    --image-family=ubuntu-2004-lts \
    --image-project=ubuntu-os-cloud \
    --boot-disk-size=50GB \
    --metadata-from-file startup-script=startup.sh

# Startup script (startup.sh)
#!/bin/bash
apt-get update
apt-get install -y docker.io docker-compose git

# Clone and setup
git clone https://github.com/yourorg/crush-deployment.git /opt/crush
cd /opt/crush
docker-compose up -d
```

**Cloud Run Deployment**:
```yaml
# cloudbuild.yaml
steps:
  - name: 'gcr.io/cloud-builders/docker'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/crush:$COMMIT_SHA', '.']
  
  - name: 'gcr.io/cloud-builders/docker'
    args: ['push', 'gcr.io/$PROJECT_ID/crush:$COMMIT_SHA']
  
  - name: 'gcr.io/cloud-builders/gcloud'
    args:
      - 'run'
      - 'deploy'
      - 'crush'
      - '--image'
      - 'gcr.io/$PROJECT_ID/crush:$COMMIT_SHA'
      - '--region'
      - 'us-central1'
      - '--platform'
      - 'managed'
      - '--allow-unauthenticated'
```

### 3. Azure Deployment

**Container Instances**:
```yaml
# azure-deploy.yaml
apiVersion: 2019-12-01
location: eastus
name: crush-container-group
properties:
  containers:
  - name: crush
    properties:
      image: crush:latest
      resources:
        requests:
          cpu: 1
          memoryInGb: 2
      environmentVariables:
      - name: OPENAI_API_KEY
        secureValue: your-api-key
      volumeMounts:
      - name: data-volume
        mountPath: /data
  
  volumes:
  - name: data-volume
    azureFile:
      shareName: crush-data
      storageAccountName: crushstorage
      storageAccountKey: your-storage-key
  
  osType: Linux
  restartPolicy: Always
```

## 📊 Monitoring & Observability

### 1. Logging Configuration

**Structured Logging**:
```json
{
  "options": {
    "debug": false,
    "log_level": "info",
    "log_format": "json",
    "log_file": "/var/log/crush/crush.log"
  }
}
```

**Log Aggregation (ELK Stack)**:
```yaml
# docker-compose.logging.yml
version: '3.8'

services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.5.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
    volumes:
      - elasticsearch-data:/usr/share/elasticsearch/data
    ports:
      - "9200:9200"

  logstash:
    image: docker.elastic.co/logstash/logstash:8.5.0
    volumes:
      - ./logstash.conf:/usr/share/logstash/pipeline/logstash.conf
    ports:
      - "5044:5044"
    depends_on:
      - elasticsearch

  kibana:
    image: docker.elastic.co/kibana/kibana:8.5.0
    ports:
      - "5601:5601"
    depends_on:
      - elasticsearch

  filebeat:
    image: docker.elastic.co/beats/filebeat:8.5.0
    volumes:
      - ./filebeat.yml:/usr/share/filebeat/filebeat.yml
      - /var/log/crush:/var/log/crush:ro
    depends_on:
      - logstash

volumes:
  elasticsearch-data:
```

### 2. Metrics Collection

**Prometheus Configuration**:
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'crush'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 30s
```

**Grafana Dashboard**:
```json
{
  "dashboard": {
    "title": "Crush AI Assistant Metrics",
    "panels": [
      {
        "title": "API Requests",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(crush_api_requests_total[5m])"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(crush_response_time_seconds_bucket[5m]))"
          }
        ]
      }
    ]
  }
}
```

## 🔒 Security Considerations

### 1. Network Security

**Firewall Rules**:
```bash
# Allow SSH
ufw allow 22/tcp

# Allow application port (if needed)
ufw allow 8080/tcp

# Enable firewall
ufw enable
```

**TLS Configuration**:
```nginx
server {
    listen 443 ssl http2;
    server_name crush.yourdomain.com;
    
    ssl_certificate /etc/ssl/certs/crush.crt;
    ssl_certificate_key /etc/ssl/private/crush.key;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

### 2. Secrets Management

**HashiCorp Vault Integration**:
```bash
# Install Vault agent
vault agent -config=vault-agent.hcl

# Vault agent configuration
vault {
  address = "https://vault.yourdomain.com"
}

auto_auth {
  method "aws" {
    mount_path = "auth/aws"
    config = {
      type = "iam"
      role = "crush-role"
    }
  }
}

template {
  source      = "/etc/crush/crush.json.tpl"
  destination = "/etc/crush/crush.json"
  command     = "systemctl reload crush"
}
```

### 3. Access Control

**RBAC Configuration**:
```yaml
# rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: crush
  name: crush-operator
rules:
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "update"]
```

## 📋 Deployment Checklist

### Pre-Deployment
- [ ] **Environment Setup**: Verify system requirements
- [ ] **Configuration**: Validate configuration files
- [ ] **Secrets**: Secure API key management
- [ ] **Dependencies**: Install required LSP servers
- [ ] **Network**: Configure firewall and network access
- [ ] **Storage**: Set up persistent storage
- [ ] **Monitoring**: Configure logging and metrics

### Deployment
- [ ] **Build**: Create deployment artifacts
- [ ] **Deploy**: Execute deployment process
- [ ] **Verify**: Test basic functionality
- [ ] **Health Check**: Verify all services are healthy
- [ ] **Integration**: Test external integrations
- [ ] **Performance**: Validate performance metrics

### Post-Deployment
- [ ] **Documentation**: Update deployment documentation
- [ ] **Monitoring**: Set up alerts and dashboards
- [ ] **Backup**: Configure backup procedures
- [ ] **Maintenance**: Schedule maintenance windows
- [ ] **Training**: Train team on deployment procedures

---

*This deployment guide provides comprehensive coverage of various deployment scenarios. Choose the approach that best fits your organization's requirements and infrastructure.*
