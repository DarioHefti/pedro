<#
.SYNOPSIS
    Provisions an Azure OpenAI Service resource with a model deployment and
    assigns an Entra ID principal the "Cognitive Services OpenAI User" role,
    compatible with pedro's InteractiveBrowserCredential SSO flow.

.DESCRIPTION
    Idempotent: safe to re-run. Skips any resource that already exists.

    Prerequisites:
      - Azure CLI (az) >= 2.50: https://learn.microsoft.com/cli/azure/install-azure-cli
      - Logged in via: az login
      - Subscription with Azure OpenAI access approved by Microsoft

.PARAMETER SubscriptionId
    (Required) Azure Subscription ID to deploy into.

.PARAMETER Location
    (Optional) Azure region. Default: switzerlandnorth
    gpt-5.2 availability: switzerlandnorth, swedencentral, eastus2, polandcentral, etc.

.PARAMETER ResourceGroupName
    (Optional) Resource group name. Default: rg-pedro-openai

.PARAMETER OpenAIResourceName
    (Optional) Cognitive Services account name. Default: pedro-openai-<random4>
    Must be globally unique; only lowercase letters, digits, and hyphens.

.PARAMETER DeploymentName
    (Optional) Name for the model deployment. Default: gpt-5.2

.PARAMETER ModelName
    (Optional) Azure OpenAI model name. Default: gpt-5.2

.PARAMETER ModelVersion
    (Optional) Model version. Default: 2025-12-11

.PARAMETER ModelSkuName
    (Optional) Deployment SKU. Default: GlobalStandard

.PARAMETER CapacityTPM
    (Optional) Tokens-per-minute quota in thousands. Default: 100 (= 100K TPM)

.PARAMETER EntraGroupObjectId
    (Optional) Object ID of an Entra ID group whose members should get access.
    Mutually exclusive with -AssigneeObjectId. Use this for the common case of
    granting access to a group (e.g. "All Company" or a team group).

.PARAMETER AssigneeObjectId
    (Optional) Object ID of a User, Group, or Service Principal to assign the
    role to. Required if -EntraGroupObjectId is not provided.

.PARAMETER AssigneePrincipalType
    (Optional) Type of the principal in -AssigneeObjectId.
    Accepted values: User | Group | ServicePrincipal. Default: Group

.PARAMETER RoleName
    (Optional) Azure built-in role to assign. Default: Cognitive Services OpenAI User

.EXAMPLE
    # Typical usage -- grant access to an Entra group
    .\setup-azure-openai.ps1 `
        -SubscriptionId      "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" `
        -EntraGroupObjectId  "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy"

.EXAMPLE
    # Grant access to a single user, custom names
    .\setup-azure-openai.ps1 `
        -SubscriptionId        "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" `
        -OpenAIResourceName    "mycompany-openai" `
        -ResourceGroupName     "rg-ai-prod" `
        -Location              "swedencentral" `
        -AssigneeObjectId      "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz" `
        -AssigneePrincipalType "User"
#>

[CmdletBinding(DefaultParameterSetName = "ByGroup")]
param(
    # ---- Infrastructure ----
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}$')]
    [string]$SubscriptionId,

    [Parameter(Mandatory = $false)]
    [string]$Location = "switzerlandnorth",

    [Parameter(Mandatory = $false)]
    [string]$ResourceGroupName = "rg-pedro-openai",

    [Parameter(Mandatory = $false)]
    [ValidatePattern('^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$')]
    [string]$OpenAIResourceName = "",

    # ---- Model deployment ----
    [Parameter(Mandatory = $false)]
    [string]$DeploymentName = "gpt-5.2",

    [Parameter(Mandatory = $false)]
    [string]$ModelName = "gpt-5.2",

    [Parameter(Mandatory = $false)]
    [string]$ModelVersion = "2025-12-11",

    [Parameter(Mandatory = $false)]
    [string]$ModelSkuName = "GlobalStandard",

    [Parameter(Mandatory = $false)]
    [ValidateRange(1, 2000)]
    [int]$CapacityTPM = 100,

    # ---- RBAC ----
    [Parameter(Mandatory = $false, ParameterSetName = "ByGroup")]
    [ValidatePattern('^[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}$')]
    [string]$EntraGroupObjectId = "",

    [Parameter(Mandatory = $false, ParameterSetName = "ByAssignee")]
    [ValidatePattern('^[0-9a-f]{8}-([0-9a-f]{4}-){3}[0-9a-f]{12}$')]
    [string]$AssigneeObjectId = "",

    [Parameter(Mandatory = $false, ParameterSetName = "ByAssignee")]
    [ValidateSet("User", "Group", "ServicePrincipal")]
    [string]$AssigneePrincipalType = "Group",

    [Parameter(Mandatory = $false)]
    [string]$RoleName = "Cognitive Services OpenAI User"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------------------
# Resolve which assignee to use and validate inputs
# ---------------------------------------------------------------------------

$ResolvedAssigneeId   = ""
$ResolvedPrincipalType = ""

if ($EntraGroupObjectId -ne "") {
    $ResolvedAssigneeId    = $EntraGroupObjectId
    $ResolvedPrincipalType = "Group"
}
elseif ($AssigneeObjectId -ne "") {
    $ResolvedAssigneeId    = $AssigneeObjectId
    $ResolvedPrincipalType = $AssigneePrincipalType
}
else {
    Write-Error @"
No RBAC assignee provided. Please supply one of:

  -EntraGroupObjectId  "<object-id>"          # Recommended: an Entra ID group
  -AssigneeObjectId    "<object-id>" [-AssigneePrincipalType User|Group|ServicePrincipal]

To find an Entra group object ID:
  az ad group list --display-name "My Group" --query "[].id" -o tsv

To find a user object ID:
  az ad user show --id user@example.com --query id -o tsv
"@
    exit 1
}

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

function Write-Step([string]$msg) {
    Write-Host ""
    Write-Host ">> $msg" -ForegroundColor Cyan
}

function Write-Ok([string]$msg) {
    Write-Host "   [OK]   $msg" -ForegroundColor Green
}

function Write-Skip([string]$msg) {
    Write-Host "   [SKIP] $msg" -ForegroundColor Yellow
}

function Write-Info([string]$msg) {
    Write-Host "   [..]   $msg" -ForegroundColor Gray
}

function Write-Warn([string]$msg) {
    Write-Host "   [WARN] $msg" -ForegroundColor Yellow
}

# Run az CLI and throw a descriptive error on failure.
# Returns raw string output so callers can decide whether to parse JSON.
function Invoke-AzRaw {
    [CmdletBinding()]
    param([Parameter(ValueFromRemainingArguments)][string[]]$AzArgs)

    $output = az @AzArgs 2>&1
    if ($LASTEXITCODE -ne 0) {
        $cmd = "az $($AzArgs -join ' ')"
        throw "Command failed (exit $LASTEXITCODE):`n  $cmd`n`nOutput:`n$output"
    }
    return $output
}

function Invoke-AzJson {
    [CmdletBinding()]
    param([Parameter(ValueFromRemainingArguments)][string[]]$AzArgs)

    $raw = Invoke-AzRaw @AzArgs
    return $raw | ConvertFrom-Json
}

# ---------------------------------------------------------------------------
# 1. Prerequisites
# ---------------------------------------------------------------------------

Write-Step "Checking prerequisites"

if (-not (Get-Command az -ErrorAction SilentlyContinue)) {
    Write-Error "Azure CLI (az) is not installed. Install it from: https://learn.microsoft.com/cli/azure/install-azure-cli"
    exit 1
}

$azVersion = az version --query '"azure-cli"' -o tsv 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error "Could not determine az CLI version. Ensure az is functional."
    exit 1
}
Write-Ok "Azure CLI $azVersion"

$account = $null
try {
    $account = Invoke-AzJson account show
}
catch {
    Write-Error "Not logged in to Azure CLI. Run: az login"
    exit 1
}
Write-Ok "Signed in as: $($account.user.name)  |  Tenant: $($account.tenantId)"

# ---------------------------------------------------------------------------
# 2. Set and verify subscription
# ---------------------------------------------------------------------------

Write-Step "Subscription: $SubscriptionId"

try {
    Invoke-AzRaw account set --subscription $SubscriptionId | Out-Null
}
catch {
    Write-Error "Failed to set subscription '$SubscriptionId'. Verify the ID and that you have access.`n$_"
    exit 1
}

$sub = Invoke-AzJson account show
if ($sub.id -ne $SubscriptionId) {
    Write-Error "Subscription mismatch after 'az account set'. Expected '$SubscriptionId', got '$($sub.id)'."
    exit 1
}
Write-Ok "$($sub.name)  ($SubscriptionId)"

# ---------------------------------------------------------------------------
# 3. Generate resource name if not provided
# ---------------------------------------------------------------------------

if ($OpenAIResourceName -eq "") {
    $suffix = -join (1..4 | ForEach-Object { [char](Get-Random -Minimum 97 -Maximum 123) })
    $OpenAIResourceName = "pedro-openai-$suffix"
    Write-Info "No -OpenAIResourceName provided. Using: $OpenAIResourceName"
}

# ---------------------------------------------------------------------------
# 4. Resource group (idempotent)
# ---------------------------------------------------------------------------

Write-Step "Resource group: $ResourceGroupName"

$rgExists = az group exists --name $ResourceGroupName 2>&1
if ($rgExists -eq "true") {
    Write-Skip "'$ResourceGroupName' already exists"
}
else {
    try {
        Invoke-AzRaw group create --name $ResourceGroupName --location $Location | Out-Null
        Write-Ok "Created '$ResourceGroupName' in $Location"
    }
    catch {
        Write-Error "Failed to create resource group '$ResourceGroupName':`n$_"
        exit 1
    }
}

# ---------------------------------------------------------------------------
# 5. Azure OpenAI Cognitive Services account (idempotent)
# ---------------------------------------------------------------------------

Write-Step "Azure OpenAI resource: $OpenAIResourceName"

$existingAccounts = az cognitiveservices account list `
    --resource-group $ResourceGroupName `
    --query "[?name=='$OpenAIResourceName']" `
    -o json 2>&1 | ConvertFrom-Json

$oaiAccount = $null
if ($existingAccounts.Count -gt 0) {
    Write-Skip "'$OpenAIResourceName' already exists -- reusing"
    $oaiAccount = $existingAccounts[0]
}
else {
    Write-Info "Creating Cognitive Services account (kind=OpenAI, sku=S0)..."
    try {
        $oaiAccount = Invoke-AzJson cognitiveservices account create `
            --name $OpenAIResourceName `
            --resource-group $ResourceGroupName `
            --kind "OpenAI" `
            --sku "S0" `
            --location $Location `
            --custom-domain $OpenAIResourceName `
            --yes
        Write-Ok "Created '$OpenAIResourceName'"
    }
    catch {
        Write-Error "Failed to create Azure OpenAI resource '$OpenAIResourceName':`n$_"
        exit 1
    }
}

$OAIResourceId = $oaiAccount.id
$Endpoint      = $oaiAccount.properties.endpoint
if ([string]::IsNullOrWhiteSpace($Endpoint)) {
    $Endpoint = "https://$OpenAIResourceName.openai.azure.com/"
}
Write-Info "Endpoint: $Endpoint"

# ---------------------------------------------------------------------------
# 6. Model deployment (idempotent)
# ---------------------------------------------------------------------------

Write-Step "Model deployment: '$DeploymentName'  ($ModelName $ModelVersion / $ModelSkuName / ${CapacityTPM}K TPM)"

$existingDeployments = az cognitiveservices account deployment list `
    --name $OpenAIResourceName `
    --resource-group $ResourceGroupName `
    --query "[?name=='$DeploymentName']" `
    -o json 2>&1 | ConvertFrom-Json

if ($existingDeployments.Count -gt 0) {
    Write-Skip "Deployment '$DeploymentName' already exists"
}
else {
    Write-Info "Deploying model -- this may take 1-2 minutes..."

    $deployCmd = @(
        "cognitiveservices", "account", "deployment", "create",
        "--name",            $OpenAIResourceName,
        "--resource-group",  $ResourceGroupName,
        "--deployment-name", $DeploymentName,
        "--model-name",      $ModelName,
        "--model-version",   $ModelVersion,
        "--model-format",    "OpenAI",
        "--sku-name",        $ModelSkuName,
        "--sku-capacity",    $CapacityTPM
    )

    try {
        Invoke-AzRaw @deployCmd | Out-Null
        Write-Ok "Deployed '$DeploymentName'  ($ModelName $ModelVersion, ${CapacityTPM}K TPM)"
    }
    catch {
        Write-Host ""
        Write-Host "   [FAIL] Model deployment failed." -ForegroundColor Red
        Write-Host "          Failing command:" -ForegroundColor Red
        Write-Host "            az $($deployCmd -join ' ')" -ForegroundColor White
        Write-Host ""
        Write-Error $_
        exit 1
    }
}

# ---------------------------------------------------------------------------
# 7. RBAC -- Cognitive Services OpenAI User scoped to the OAI resource
#
#    Least-privilege: scoped to the OpenAI resource itself, not the RG.
#    Principal is whatever was passed in (-EntraGroupObjectId or -AssigneeObjectId).
#
#    Best-effort: try to validate the principal exists in Entra ID before
#    assigning. If Graph is locked down we warn and proceed anyway.
# ---------------------------------------------------------------------------

Write-Step "RBAC: '$RoleName'  ->  $ResolvedPrincipalType '$ResolvedAssigneeId'"

$RbacScope = $OAIResourceId   # resource-level scope (least privilege)

# -- Best-effort principal validation --
Write-Info "Validating assignee in Entra ID (best-effort)..."
$principalValidated = $false
try {
    if ($ResolvedPrincipalType -eq "Group") {
        $principalCheck = az ad group show --group $ResolvedAssigneeId -o json 2>&1
    }
    elseif ($ResolvedPrincipalType -eq "User") {
        $principalCheck = az ad user show --id $ResolvedAssigneeId -o json 2>&1
    }
    else {
        $principalCheck = az ad sp show --id $ResolvedAssigneeId -o json 2>&1
    }

    if ($LASTEXITCODE -eq 0) {
        $principalObj = $principalCheck | ConvertFrom-Json
        $displayName  = if ($principalObj.displayName) { $principalObj.displayName } else { "(display name unavailable)" }
        Write-Ok "Principal found: $displayName"
        $principalValidated = $true
    }
    else {
        Write-Warn "Could not validate principal (Graph may be restricted). Proceeding with provided object ID."
    }
}
catch {
    Write-Warn "Principal validation skipped (Graph call failed). Proceeding with provided object ID."
}

# -- Check if assignment already exists --
$existingAssignment = az role assignment list `
    --scope $RbacScope `
    --role $RoleName `
    --query "[?principalId=='$ResolvedAssigneeId']" `
    -o json 2>&1 | ConvertFrom-Json

if ($existingAssignment.Count -gt 0) {
    Write-Skip "Role assignment already exists for '$ResolvedAssigneeId'"
}
else {
    try {
        Invoke-AzRaw role assignment create `
            --role $RoleName `
            --assignee-object-id $ResolvedAssigneeId `
            --assignee-principal-type $ResolvedPrincipalType `
            --scope $RbacScope | Out-Null
        Write-Ok "Assigned '$RoleName' to $ResolvedPrincipalType '$ResolvedAssigneeId'"
        Write-Info "Scope: $RbacScope"
    }
    catch {
        Write-Host ""
        Write-Host "   [FAIL] Role assignment failed." -ForegroundColor Red
        Write-Host "          If this is a permissions issue, re-run as an Owner/User Access Administrator." -ForegroundColor Yellow
        Write-Host "          Or ask an admin to run:" -ForegroundColor Yellow
        Write-Host ""
        Write-Host "            az role assignment create ``" -ForegroundColor White
        Write-Host "              --role '$RoleName' ``" -ForegroundColor White
        Write-Host "              --assignee-object-id '$ResolvedAssigneeId' ``" -ForegroundColor White
        Write-Host "              --assignee-principal-type '$ResolvedPrincipalType' ``" -ForegroundColor White
        Write-Host "              --scope '$RbacScope'" -ForegroundColor White
        Write-Host ""
        Write-Error $_
        exit 1
    }
}

# ---------------------------------------------------------------------------
# 8. Summary
# ---------------------------------------------------------------------------

Write-Host ""
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host "  Done. Configure pedro with:" -ForegroundColor Cyan
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Provider      : azure"                        -ForegroundColor White
Write-Host "  Endpoint      : $Endpoint"                   -ForegroundColor Green
Write-Host "  Deployment    : $DeploymentName"             -ForegroundColor Green
Write-Host ""
Write-Host "  Resource      : $OpenAIResourceName"         -ForegroundColor Gray
Write-Host "  Resource Group: $ResourceGroupName"          -ForegroundColor Gray
Write-Host "  RBAC Scope    : $RbacScope"                  -ForegroundColor Gray
Write-Host "  Assignee      : $ResolvedPrincipalType $ResolvedAssigneeId" -ForegroundColor Gray
Write-Host ""
Write-Host "  In pedro -> Settings -> Provider: azure"     -ForegroundColor Gray
Write-Host "  Paste Endpoint + Deployment above, sign in." -ForegroundColor Gray
Write-Host ""
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host ""
