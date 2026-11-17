# Sandbox providers, November 2026

15 boat.dev-like products, read from their official docs on 2026-11-12. "No" means the docs don't show it, not that it can't exist. Docker cells were read through a summarizer, so they are less exact.

## Matrix

| Provider | Isolation | Native SSH | Fork | Lifecycle webhooks | Keeps real keys out | Subscription login documented | Built-in agent harness | Self-host |
|---|---|---|---|---|---|---|---|---|
| boat.dev | VM | Yes | Yes | Yes | Placeholder + base URL proxy | Yes (Claude, ChatGPT, Grok) | Yes, 7 agents | No |
| E2B | Firecracker | DIY (websocat) | Yes | Yes (HMAC) | Header injection by host (beta) | No | Template only | Yes, Apache-2.0 |
| Daytona | Container, or VM | Yes | VM only | Yes | Placeholder, host-pinned header swap | No | No (Claude preinstalled) | BYOC |
| Modal | gVisor, or VM | No | Via snapshot | No | Header injection (beta) | No | No | No |
| Runloop | VM | Yes | Via disk snapshot | No | Gateway URL + token | No | Claude protocol broker | No |
| Vercel Sandbox | Firecracker | CLI shell only | Yes | No | Header transform by domain | No | `sandbox create claude\|codex\|...` | SDK only |
| Cloudflare Sandbox | Firecracker | `wrangler` only | No | No | DIY Worker intercept | No | Templates | No |
| Fly.io Sprites | microVM | Via `sprite proxy` | No | No | Gateway URL (connectors) | No | Plugins | No |
| AWS AgentCore | microVM | No | No | Agent-loop hooks only | No (token vault into agent code) | No | Own harness only | No |
| Freestyle | VM | Yes | Claimed | No | Edge TLS rule, placeholder | Yes (Claude `setup-token`) | No | No |
| Beam | microVM, or container | Yes | Yes | Partial | Placeholder, host-scoped | No | No | Yes (Beta9) |
| Blaxel | VM | No | Preview | No | Proxy injection (preview) | No | No | No |
| Northflank | microVM / gVisor | Yes | No | No | No | Sign-in inside box | Yes, 7 agents | BYOC |
| Docker Sandboxes | microVM | Editor SSH | No | No | Host proxy, sentinel value | Yes (Codex OAuth) | Yes, 10 agents | Local `sbx` free |
| Upstash Box | Container | Yes | Via snapshot | No | Header attach by host | No | Yes, 4 agents | No |

## What this changes

| Finding | Count | Consequence for TaskWing |
|---|---|---|
| Every provider supports create, exec, file copy, list, destroy | 15 / 15 (Cloudflare needs a Worker shim) | That is the Box contract. Nothing else is required. |
| Native SSH | 8 / 15 | SSH can't be the lowest common box. Exec + file copy is. |
| Keeps real keys out | 13 / 15 | Use the provider's own injection when present. TaskWing's own proxy is the fallback, not the default. |
| Two proxy shapes | Header swap (most) vs gateway base URL (Runloop, Sprites, Cloudflare/Vercel AI Gateway) | The Agent side must set both a placeholder key and a base URL, so either shape works. |
| Subscription login behind a proxy | 3 / 15 (boat, Freestyle, Docker) | API keys work everywhere. Subscriptions are a per-provider capability. |
| Lifecycle webhooks | 3 / 15 (boat, E2B, Daytona) | Box state comes from `list()` reconcile. Webhooks only speed it up. |
| Fork | ~7 / 15, several preview | Optional. Fallback: create every box from the same image or snapshot. |
| Built-in agent harness | 5 / 15, all different | Don't use them. TaskWing runs the agent CLI itself over exec (ACP), the same on every provider. |
| Self-hostable | E2B, Beam (+ Docker local) | A free local backend exists for development and tests. |

## Box adapter shape

```
required   create(image, env, network) · exec(stream) · copy in/out · list(labels) · destroy
optional   ssh · fork · snapshot · pause/resume · webhooks · inject_header(host, secret) · gateway_url
```

The core has a fallback for every optional item. An adapter declares what it has, nothing more.

## Hyperscaler primitives (you run them, not SaaS)

| Primitive | What | How to use | Cost | Gaps |
|---|---|---|---|---|
| AWS Nitro Sandboxes (blog, 10 Nov 2026) | A lightweight VM per container, own kernel, CPU and memory split by the Nitro Hypervisor in hardware, no nested hypervisor. "Near-native" speed, no cold start number. | EKS: `runtimeClassName: isolated-sandbox` or `confidential-sandbox`. ECS Managed Instances: `trustedExecutionConfiguration`. EC2: direct, with Firecracker. Every Nitro Enclaves instance type, all commercial regions. | No extra charge: you pay for the EC2 instances. | No snapshot, fork, pause or credential injection mentioned. Network policy and security groups are enforced outside the sandbox, so egress can be pinned to TaskWing's proxy. |
| Google Agent Substrate + gVisor | Donated to CNCF (Nov 2026), runs on any conformant Kubernetes | Kubernetes | Your cluster | Sandbox status at CNCF; early |
| Kubernetes agent-sandbox | gVisor or Kata per workload, Apache-2.0, v1.0.4 | Kubernetes | Your cluster | Not read |

What this means:

- **One Kubernetes Box adapter covers all three.** A pod with a `runtimeClassName` is the box, on EKS (Nitro), GKE or any cluster (gVisor, Kata). That puts a free, self-run backend next to the SaaS ones.
- **Isolation is becoming free.** AWS gives hardware isolation away with EC2. Sandbox SaaS will have to compete on extras (fork, snapshots, key proxies, agent runners), which is where value moves up the stack toward TaskWing's layer.
- **Confidential mode protects the wrong side for us.** It hides the box's memory from the host operator, and attestation-gated KMS releases keys only into a measured box. TaskWing's threat is the agent inside the box, not the host, so rule 3 (no real key in the box) still applies.

Source: [Nitro Sandboxes, AWS Compute Blog](https://aws.amazon.com/blogs/compute/nitro-sandboxes-hardware-isolation-for-every-eks-pod-ecs-task-and-ec2-workload/). Fetched page was truncated; later sections unread.

## Watch

- Runloop's docs have a "Transition from Runloop to Freestyle" page. Unread; it may signal consolidation.
- Most credential proxies are beta or preview (E2B, Modal, Blaxel, Docker cloud).

## Doc indexes

boat `docs.boat.dev/llms.txt` · E2B `docs.e2b.dev/llms-full.txt` · Daytona `daytona.io/docs/llms.txt` · Modal `modal.com/llms-full.txt` · Runloop `docs.runloop.ai/llms.txt` · Vercel `vercel.com/docs/sandbox` · Cloudflare `developers.cloudflare.com/sandbox/` · Sprites `docs.fly.io/llms.txt` · AgentCore `docs.aws.amazon.com/bedrock-agentcore/latest/devguide/` · Freestyle `freestyle.sh/docs/llms.txt` · Beam `docs.beam.cloud/llms.txt` · Blaxel `docs.blaxel.ai/llms.txt` · Northflank `northflank.com/docs/llms.txt` · Docker `docs.docker.com/ai/sandboxes/` · Upstash `upstash.com/docs/llms.txt`

Not read: Morph Cloud, Hopx, CodeSandbox SDK (403), Koyeb (unreachable), Kubernetes agent-sandbox, Alibaba OpenSandbox.
