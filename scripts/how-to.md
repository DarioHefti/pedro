# Azure OpenAI Setup — How To

Three scripts, run in order. Each one is idempotent (safe to re-run).

## Prerequisites

| Requirement | Used by |
|---|---|
| [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli) — `az login` | scripts 2 & 3 |
| [Microsoft.Graph PowerShell module](https://learn.microsoft.com/powershell/microsoftgraph/installation) — `Install-Module Microsoft.Graph -Scope CurrentUser` | scripts 1 & 2 |
| Entra ID **Groups Administrator** or **Global Administrator** role | script 1 |
| Entra ID **P1 or P2** license (required for dynamic groups) | script 1 |
| Azure **Owner** or **User Access Administrator** on the subscription | script 3 |
| Microsoft-approved access to **gpt-5.2** on your subscription | script 3 |

---

## Step 1 — Create the dynamic Entra group

```powershell
.\create-entra-group.ps1
```

Creates a security group called `azure-openai-users` with the dynamic membership rule:

```
(user.userType -eq "Member")
```

This automatically includes all internal users and excludes guests. Membership is maintained by Entra ID — no manual adding/removing.

**Output:** the Group Object ID you need for step 3.

> Dynamic membership can take a few minutes to populate after creation.

---

## Step 2 (optional) — Look up an existing group

If the group already exists and you just need its Object ID:

```powershell
.\get-entra-group-id.ps1 -DisplayName "azure-openai-users"
```

Supports partial names — lists all matches if more than one is found. Works on any group type, not just dynamic ones.

---

## Step 3 — Provision Azure OpenAI

```powershell
.\setup-azure-openai.ps1 `
    -SubscriptionId     "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" `
    -EntraGroupObjectId "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy"
```

Does the following:
1. Creates resource group `rg-pedro-openai` in `switzerlandnorth`
2. Creates an Azure OpenAI Cognitive Services account
3. Deploys model `gpt-5.2` (version `2025-12-11`, 100K TPM, GlobalStandard)
4. Assigns the `Cognitive Services OpenAI User` role to the group, scoped to the OpenAI resource

**Output:** the **Endpoint URL** and **Deployment name** to paste into pedro's Settings.

### All parameters

| Parameter | Default | Notes |
|---|---|---|
| `-SubscriptionId` | — | Required |
| `-EntraGroupObjectId` | — | Required (or use `-AssigneeObjectId`) |
| `-AssigneeObjectId` + `-AssigneePrincipalType` | — | Alternative to group; type: `User\|Group\|ServicePrincipal` |
| `-Location` | `switzerlandnorth` | Also supported: `swedencentral`, `eastus2` |
| `-ResourceGroupName` | `rg-pedro-openai` | |
| `-OpenAIResourceName` | `pedro-openai-<random>` | Must be globally unique |
| `-DeploymentName` | `gpt-5.2` | |
| `-ModelName` | `gpt-5.2` | |
| `-ModelVersion` | `2025-12-11` | |
| `-ModelSkuName` | `GlobalStandard` | |
| `-CapacityTPM` | `100` (= 100K TPM) | |
| `-RoleName` | `Cognitive Services OpenAI User` | |

---

## Configure pedro

After step 3, open pedro → **Settings** → set Provider to **azure**, then paste:

- **Endpoint:** `https://<resource-name>.openai.azure.com/`
- **Deployment:** `gpt-5.2`

Click **Sign in** — a browser window opens for Entra ID authentication. No API key needed.
