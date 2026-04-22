# 🛒 ECommerce Microservices — EKS Practice Project

A production-style **ECommerce platform** built with **4 microservices**, each in a different language. Designed specifically to practise **AWS EKS, Docker, and DevOps** workflows.

---

## 🏗️ Architecture Overview

```
                         ┌─────────────────────────────┐
                         │      AWS ALB Ingress         │
                         │  (internet-facing, HTTPS)    │
                         └──────────────┬──────────────┘
                                        │
              ┌──────────────┬──────────┴──────────┬──────────────┐
              ▼              ▼                      ▼              ▼
    ┌─────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐
    │ user-service│  │product-service│  │ order-service│  │notification-svc  │
    │  (Node.js)  │  │   (Java 17)  │  │  (Python)    │  │     (Go)         │
    │  Port 3000  │  │  Port 8080   │  │  Port 8000   │  │   Port 9000      │
    └──────┬──────┘  └──────┬───────┘  └──────┬───────┘  └──────────────────┘
           │                │                  │
        MongoDB           MySQL            PostgreSQL
```

---

## 📦 Microservices

| Service               | Language    | Port | Database   | Description                    |
|-----------------------|-------------|------|------------|--------------------------------|
| **user-service**      | Node.js 18  | 3000 | MongoDB    | Register, login, JWT auth      |
| **product-service**   | Java 17     | 8080 | MySQL 8    | Product CRUD, stock management |
| **order-service**     | Python 3.11 | 8000 | PostgreSQL | Order lifecycle management     |
| **notification-service** | Go 1.21  | 9000 | In-memory  | Email/SMS/Push notifications   |

---

## 📁 Project Structure

```
ecommerce-microservices/
├── user-service/               # Node.js + Express + MongoDB
│   ├── src/index.js
│   ├── package.json
│   └── Dockerfile
├── product-service/            # Java 17 + Spring Boot + MySQL
│   ├── src/main/java/...
│   ├── pom.xml
│   └── Dockerfile
├── order-service/              # Python + FastAPI + PostgreSQL
│   ├── src/main.py
│   ├── requirements.txt
│   └── Dockerfile
├── notification-service/       # Go + gorilla/mux (in-memory)
│   ├── src/main.go
│   ├── go.mod
│   └── Dockerfile
├── eks/                        # Kubernetes manifests for EKS
│   ├── namespace/namespace.yaml
│   ├── user-service/deployment.yaml
│   ├── product-service/deployment.yaml
│   ├── order-service/deployment.yaml
│   ├── notification-service/deployment.yaml
│   └── ingress/ingress.yaml
├── .github/workflows/deploy.yml   # CI/CD pipeline
└── docker-compose.yml             # Local development
```

---

## 🚀 Quick Start — Local (Docker Compose)

### Prerequisites
- Docker & Docker Compose installed

### Run all services locally
```bash
cd ecommerce-microservices
docker-compose up --build
```

### API Endpoints (local)
| Service              | Base URL                          |
|----------------------|-----------------------------------|
| user-service         | http://localhost:3000             |
| product-service      | http://localhost:8080             |
| order-service        | http://localhost:8000             |
| notification-service | http://localhost:9000             |

### Health checks
```bash
curl http://localhost:3000/health
curl http://localhost:8080/api/products/health
curl http://localhost:8000/health
curl http://localhost:9000/health
```

---

## ☁️ AWS EKS Deployment

### Step 1 — Prerequisites
```bash
# Install tools
brew install awscli kubectl eksctl helm

# Configure AWS CLI
aws configure
# Enter: AWS Access Key, Secret Key, Region (e.g. ap-south-1), output format json
```

### Step 2 — Create EKS Cluster
```bash
eksctl create cluster \
  --name ecommerce-eks \
  --region ap-south-1 \
  --nodegroup-name standard-workers \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 2 \
  --nodes-max 5 \
  --managed
```
> ⏱️ This takes ~15 minutes.

### Step 3 — Create ECR Repositories
```bash
REGION=ap-south-1
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

for svc in user-service product-service order-service notification-service; do
  aws ecr create-repository --repository-name $svc --region $REGION
done

echo "ECR Registry: $ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com"
```

### Step 4 — Build & Push Docker Images
```bash
REGION=ap-south-1
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REGISTRY=$ACCOUNT_ID.dkr.ecr.$REGION.amazonaws.com

# Login to ECR
aws ecr get-login-password --region $REGION | \
  docker login --username AWS --password-stdin $ECR_REGISTRY

# Build and push each service
for svc in user-service product-service order-service notification-service; do
  docker build -t $ECR_REGISTRY/$svc:latest ./$svc
  docker push $ECR_REGISTRY/$svc:latest
done
```

### Step 5 — Update EKS manifests with your ECR registry
```bash
# Replace placeholder with your actual ECR registry URL
find eks/ -name "*.yaml" -exec sed -i \
  "s|YOUR_ECR_REGISTRY|$ECR_REGISTRY|g" {} \;
```

### Step 6 — Install AWS Load Balancer Controller
```bash
# Create IAM policy for ALB controller
curl -O https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.6.0/docs/install/iam_policy.json

aws iam create-policy \
  --policy-name AWSLoadBalancerControllerIAMPolicy \
  --policy-document file://iam_policy.json

# Install via Helm
helm repo add eks https://aws.github.io/eks-charts
helm repo update

helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=ecommerce-eks \
  --set serviceAccount.create=true
```

### Step 7 — Deploy to EKS
```bash
# Apply all manifests
kubectl apply -f eks/namespace/namespace.yaml
kubectl apply -f eks/user-service/deployment.yaml
kubectl apply -f eks/product-service/deployment.yaml
kubectl apply -f eks/order-service/deployment.yaml
kubectl apply -f eks/notification-service/deployment.yaml
kubectl apply -f eks/ingress/ingress.yaml

# Watch rollout
kubectl get pods -n ecommerce -w
```

### Step 8 — Verify Deployment
```bash
# Check pods
kubectl get pods -n ecommerce

# Check services
kubectl get svc -n ecommerce

# Get the ALB DNS name
kubectl get ingress -n ecommerce

# Test health endpoints via ALB
ALB_DNS=$(kubectl get ingress ecommerce-ingress -n ecommerce -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
curl http://$ALB_DNS/api/users/health
curl http://$ALB_DNS/api/products/health
curl http://$ALB_DNS/api/orders/health
curl http://$ALB_DNS/api/notifications/health
```

---

## 🔁 CI/CD Pipeline (GitHub Actions)

### Setup
1. Push this project to a GitHub repository
2. Add these **Secrets** in GitHub → Settings → Secrets:
   - `AWS_ACCESS_KEY_ID`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_ACCOUNT_ID`

3. Update `.github/workflows/deploy.yml`:
   - Change `AWS_REGION` to your region
   - Change `EKS_CLUSTER_NAME` to your cluster name

### Pipeline Flow
```
Push to main
    │
    ├── Build user-service image  → Push to ECR
    ├── Build product-service     → Push to ECR
    ├── Build order-service       → Push to ECR
    ├── Build notification-service→ Push to ECR
    │
    └── Deploy all to EKS (kubectl apply)
            └── Verify rollout status
```

---

## 🌐 API Reference

### User Service (Node.js) — Port 3000
| Method | Endpoint                  | Description       |
|--------|---------------------------|-------------------|
| GET    | /health                   | Health check      |
| POST   | /api/users/register       | Register user     |
| POST   | /api/users/login          | Login & get JWT   |
| GET    | /api/users                | List all users    |
| GET    | /api/users/:id            | Get user by ID    |
| PUT    | /api/users/:id            | Update user       |
| DELETE | /api/users/:id            | Delete user       |

### Product Service (Java) — Port 8080
| Method | Endpoint                        | Description          |
|--------|---------------------------------|----------------------|
| GET    | /api/products/health            | Health check         |
| GET    | /api/products                   | List all products    |
| GET    | /api/products?category=X        | Filter by category   |
| GET    | /api/products?search=X          | Search by name       |
| GET    | /api/products/:id               | Get product by ID    |
| POST   | /api/products                   | Create product       |
| PUT    | /api/products/:id               | Update product       |
| DELETE | /api/products/:id               | Delete product       |
| PATCH  | /api/products/:id/stock         | Update stock qty     |

### Order Service (Python) — Port 8000
| Method | Endpoint                        | Description          |
|--------|---------------------------------|----------------------|
| GET    | /health                         | Health check         |
| POST   | /api/orders                     | Create order         |
| GET    | /api/orders                     | List all orders      |
| GET    | /api/orders/:id                 | Get order by ID      |
| GET    | /api/orders/user/:userId        | Orders by user       |
| PUT    | /api/orders/:id                 | Update order status  |
| DELETE | /api/orders/:id                 | Cancel order         |

### Notification Service (Go) — Port 9000
| Method | Endpoint                            | Description          |
|--------|-------------------------------------|----------------------|
| GET    | /health                             | Health check         |
| POST   | /api/notifications                  | Send notification    |
| GET    | /api/notifications                  | List all             |
| GET    | /api/notifications/:id              | Get by ID            |
| GET    | /api/notifications/user/:userId     | User notifications   |
| PATCH  | /api/notifications/:id/status       | Update status        |

---

## 🧹 Cleanup

```bash
# Delete EKS resources
kubectl delete namespace ecommerce

# Delete the EKS cluster (saves AWS costs!)
eksctl delete cluster --name ecommerce-eks --region ap-south-1

# Delete ECR repositories
for svc in user-service product-service order-service notification-service; do
  aws ecr delete-repository --repository-name $svc --region ap-south-1 --force
done
```

> ⚠️ **Always run cleanup** after practising to avoid unexpected AWS charges.

---

## 💡 What You'll Learn (DevOps Skills)

- ✅ Containerisation with multi-stage Dockerfiles
- ✅ Multi-language microservice architecture
- ✅ AWS EKS cluster creation with `eksctl`
- ✅ ECR image registry management
- ✅ Kubernetes Deployments, Services, Ingress, HPA, Secrets
- ✅ AWS ALB Ingress Controller for traffic routing
- ✅ Rolling update deployment strategy
- ✅ Resource requests/limits and health probes
- ✅ GitHub Actions CI/CD pipeline to EKS
- ✅ Docker Compose for local development

---

## 🔐 Security Notes

- Secrets in the EKS manifests are **placeholders** — use **AWS Secrets Manager** or **External Secrets Operator** in real projects.
- Enable **IRSA** (IAM Roles for Service Accounts) for fine-grained pod-level AWS permissions.
- Enable **Network Policies** to restrict inter-service communication.
