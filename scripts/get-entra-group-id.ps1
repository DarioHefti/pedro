<#
.SYNOPSIS
    Looks up the Object ID of an existing Entra ID group by display name.
    Works for any group type: dynamic, assigned, security, M365.

.DESCRIPTION
    Connects to Microsoft Graph with read-only scope and queries for groups
    matching the provided display name. Prints the Object ID so you can pass
    it directly to setup-azure-openai.ps1 -EntraGroupObjectId.

    Prerequisites:
      - Microsoft.Graph module: Install-Module Microsoft.Graph -Scope CurrentUser
      - Any authenticated Entra ID user (no admin role required for read)

.PARAMETER DisplayName
    (Required) Display name of the group to look up.
    Supports partial matches -- all matches are listed if more than one found.

.EXAMPLE
    .\get-entra-group-id.ps1 -DisplayName "azure-openai-users"

.EXAMPLE
    # Partial name -- lists all groups containing "openai"
    .\get-entra-group-id.ps1 -DisplayName "openai"
#>

param(
    [Parameter(Mandatory = $true)]
    [string]$DisplayName
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

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

function Write-Info([string]$msg) {
    Write-Host "   [..]   $msg" -ForegroundColor Gray
}

function Write-Warn([string]$msg) {
    Write-Host "   [WARN] $msg" -ForegroundColor Yellow
}

# ---------------------------------------------------------------------------
# 1. Module check
# ---------------------------------------------------------------------------

Write-Step "Checking prerequisites"

$mgModule = Get-Module -ListAvailable -Name "Microsoft.Graph.Groups" -ErrorAction SilentlyContinue
if (-not $mgModule) {
    Write-Host ""
    Write-Host "   [FAIL] Microsoft.Graph module not found." -ForegroundColor Red
    Write-Host "          Install it with:" -ForegroundColor Yellow
    Write-Host "            Install-Module Microsoft.Graph -Scope CurrentUser" -ForegroundColor White
    Write-Host ""
    exit 1
}
Write-Ok "Microsoft.Graph module found (v$($mgModule[0].Version))"

# ---------------------------------------------------------------------------
# 2. Connect to Microsoft Graph (read-only)
# ---------------------------------------------------------------------------

Write-Step "Connecting to Microsoft Graph"

$requiredScope = "Group.Read.All"
$context = Get-MgContext -ErrorAction SilentlyContinue

if ($context -and ($requiredScope -in $context.Scopes)) {
    Write-Ok "Already connected as: $($context.Account)"
}
else {
    try {
        Connect-MgGraph -Scopes $requiredScope -ErrorAction Stop
        $context = Get-MgContext
        Write-Ok "Connected as: $($context.Account)"
    }
    catch {
        Write-Host ""
        Write-Host "   [FAIL] Could not connect to Microsoft Graph." -ForegroundColor Red
        Write-Error $_
        exit 1
    }
}
Write-Info "Tenant: $($context.TenantId)"

# ---------------------------------------------------------------------------
# 3. Query groups
#
#    Entra Graph filter supports 'eq' (exact) and 'startsWith' but not
#    full-text contains. We use startsWith for partial matching and then
#    filter client-side for a case-insensitive contains check.
# ---------------------------------------------------------------------------

Write-Step "Searching for groups matching: '$DisplayName'"

try {
    # startsWith gives us a server-side pre-filter; we refine client-side
    $groups = Get-MgGroup `
        -Filter "startsWith(displayName, '$($DisplayName.Substring(0, [Math]::Min(3, $DisplayName.Length)))')" `
        -Property "id,displayName,groupTypes,membershipRule,membershipRuleProcessingState,securityEnabled,description" `
        -All `
        -ErrorAction Stop |
        Where-Object { $_.DisplayName -like "*$DisplayName*" }
}
catch {
    # Fall back to listing all groups if the filter is rejected (e.g., special chars)
    Write-Warn "Server-side filter failed; falling back to client-side search (may be slow)."
    $groups = Get-MgGroup `
        -Property "id,displayName,groupTypes,membershipRule,membershipRuleProcessingState,securityEnabled,description" `
        -All `
        -ErrorAction Stop |
        Where-Object { $_.DisplayName -like "*$DisplayName*" }
}

# ---------------------------------------------------------------------------
# 4. Display results
# ---------------------------------------------------------------------------

if (-not $groups -or $groups.Count -eq 0) {
    Write-Host ""
    Write-Host "   No groups found matching '$DisplayName'." -ForegroundColor Yellow
    Write-Host "   Tip: check the spelling or try a shorter partial name." -ForegroundColor Gray
    Write-Host ""
    exit 0
}

Write-Host ""

if ($groups.Count -eq 1) {
    $g = $groups[0]
    $isDynamic  = "DynamicMembership" -in $g.GroupTypes
    $ruleState  = if ($isDynamic) { $g.MembershipRuleProcessingState } else { "n/a" }
    $rule       = if ($isDynamic -and $g.MembershipRule) { $g.MembershipRule } else { "n/a" }

    Write-Host "================================================================" -ForegroundColor Cyan
    Write-Host "  Group found:" -ForegroundColor Cyan
    Write-Host "================================================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  Name         : $($g.DisplayName)"  -ForegroundColor White
    Write-Host "  Object ID    : $($g.Id)"            -ForegroundColor Green
    Write-Host "  Dynamic      : $isDynamic"           -ForegroundColor White
    Write-Host "  Rule         : $rule"                -ForegroundColor White
    Write-Host "  Rule active  : $ruleState"           -ForegroundColor White
    Write-Host "  Security     : $($g.SecurityEnabled)" -ForegroundColor White
    if ($g.Description) {
        Write-Host "  Description  : $($g.Description)"  -ForegroundColor White
    }
    Write-Host ""
    Write-Host "  Use this Object ID with the setup script:" -ForegroundColor Cyan
    Write-Host "    .\setup-azure-openai.ps1 ``"            -ForegroundColor White
    Write-Host "      -SubscriptionId     '<subscription-id>' ``" -ForegroundColor White
    Write-Host "      -EntraGroupObjectId '$($g.Id)'"        -ForegroundColor White
    Write-Host ""
    Write-Host "================================================================" -ForegroundColor Cyan
    Write-Host ""
}
else {
    Write-Host "================================================================" -ForegroundColor Cyan
    Write-Host "  $($groups.Count) groups found matching '$DisplayName':" -ForegroundColor Cyan
    Write-Host "================================================================" -ForegroundColor Cyan
    Write-Host ""

    foreach ($g in $groups) {
        $isDynamic = "DynamicMembership" -in $g.GroupTypes
        $typeLabel = if ($isDynamic) { "dynamic  " } else { "assigned " }
        $secLabel  = if ($g.SecurityEnabled) { "security" } else { "M365    " }
        Write-Host "  $($g.Id)  |  $typeLabel  $secLabel  |  $($g.DisplayName)" -ForegroundColor White
    }

    Write-Host ""
    Write-Host "  Re-run with the exact display name to see full details." -ForegroundColor Gray
    Write-Host ""
}
