<#
.SYNOPSIS
    Creates a dynamic Entra ID security group whose members are automatically
    all internal users (userType == "Member"), excluding guests.

.DESCRIPTION
    Idempotent: if a group with the same display name already exists, the
    script reuses it and prints its Object ID instead of creating a duplicate.

    The resulting Group Object ID is meant to be passed directly to:
        .\setup-azure-openai.ps1 -EntraGroupObjectId "<id>"

    Prerequisites:
      - PowerShell 5.1+ or PowerShell 7+
      - Microsoft Graph PowerShell SDK (Microsoft.Graph module >= 2.0):
            Install-Module Microsoft.Graph -Scope CurrentUser
      - Entra ID role with group creation rights:
            "Groups Administrator" or "Global Administrator"
      - Entra ID P1 or P2 license (required for dynamic group membership)

.PARAMETER GroupDisplayName
    (Optional) Display name of the group to create. Default: azure-openai-users

.PARAMETER GroupMailNickname
    (Optional) Mail nickname (no spaces). Default: azure-openai-users

.PARAMETER MembershipRule
    (Optional) Dynamic membership rule expression.
    Default: (user.userType -eq "Member")  -- includes all internal users, excludes guests.

.PARAMETER SkipModuleCheck
    (Optional) Skip the Microsoft.Graph module presence check. Use only if you
    know the module is loaded in a custom way.

.EXAMPLE
    # Default: creates "azure-openai-users" with all-members rule
    .\create-entra-group.ps1

.EXAMPLE
    # Custom group name and rule
    .\create-entra-group.ps1 `
        -GroupDisplayName  "pedro-openai-users" `
        -MembershipRule    '(user.userType -eq "Member") and (user.department -eq "Engineering")'
#>

param(
    [Parameter(Mandatory = $false)]
    [string]$GroupDisplayName = "azure-openai-users",

    [Parameter(Mandatory = $false)]
    [ValidatePattern('^[a-zA-Z0-9_-]+$')]
    [string]$GroupMailNickname = "azure-openai-users",

    [Parameter(Mandatory = $false)]
    [string]$MembershipRule = '(user.userType -eq "Member")',

    [Parameter(Mandatory = $false)]
    [switch]$SkipModuleCheck
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

function Write-Skip([string]$msg) {
    Write-Host "   [SKIP] $msg" -ForegroundColor Yellow
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

if (-not $SkipModuleCheck) {
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
}
else {
    Write-Warn "Module check skipped (-SkipModuleCheck)"
}

# ---------------------------------------------------------------------------
# 2. Connect to Microsoft Graph
# ---------------------------------------------------------------------------

Write-Step "Connecting to Microsoft Graph"

$requiredScopes = @("Group.ReadWrite.All")

# Check if already connected with the needed scope
$context = Get-MgContext -ErrorAction SilentlyContinue
$alreadyConnected = $false

if ($context) {
    $missingScopes = $requiredScopes | Where-Object { $_ -notin $context.Scopes }
    if ($missingScopes.Count -eq 0) {
        $alreadyConnected = $true
        Write-Ok "Already connected as: $($context.Account)"
        Write-Info "Tenant: $($context.TenantId)"
    }
    else {
        Write-Info "Connected but missing scopes: $($missingScopes -join ', '). Re-connecting..."
    }
}

if (-not $alreadyConnected) {
    try {
        Connect-MgGraph -Scopes $requiredScopes -ErrorAction Stop
        $context = Get-MgContext
        Write-Ok "Connected as: $($context.Account)"
        Write-Info "Tenant: $($context.TenantId)"
    }
    catch {
        Write-Host ""
        Write-Host "   [FAIL] Could not connect to Microsoft Graph." -ForegroundColor Red
        Write-Host "          Ensure you have a browser available and Entra ID rights." -ForegroundColor Yellow
        Write-Host ""
        Write-Error $_
        exit 1
    }
}

# ---------------------------------------------------------------------------
# 3. Check for existing group (idempotent)
# ---------------------------------------------------------------------------

Write-Step "Checking for existing group: '$GroupDisplayName'"

$existingGroup = $null
try {
    # Filter server-side on displayName for efficiency
    $existingGroups = Get-MgGroup `
        -Filter "displayName eq '$GroupDisplayName'" `
        -Property "id,displayName,groupTypes,membershipRule,membershipRuleProcessingState,securityEnabled" `
        -ErrorAction Stop

    if ($existingGroups -and $existingGroups.Count -gt 0) {
        $existingGroup = $existingGroups[0]
    }
}
catch {
    Write-Warn "Could not query existing groups: $_"
    Write-Info "Proceeding to create -- will fail if a duplicate exists."
}

if ($existingGroup) {
    Write-Skip "Group '$GroupDisplayName' already exists (ID: $($existingGroup.Id))"
    Write-Info "Membership rule : $($existingGroup.MembershipRule)"
    Write-Info "Rule processing : $($existingGroup.MembershipRuleProcessingState)"
    Write-Info "Security enabled: $($existingGroup.SecurityEnabled)"

    if ($existingGroup.MembershipRule -ne $MembershipRule) {
        Write-Warn "Existing rule differs from the requested rule."
        Write-Warn "  Existing : $($existingGroup.MembershipRule)"
        Write-Warn "  Requested: $MembershipRule"
        Write-Warn "The existing rule was NOT changed. Update it manually in the Entra portal if needed."
    }

    $GroupObjectId = $existingGroup.Id
}
else {
    # ---------------------------------------------------------------------------
    # 4. Create the dynamic security group
    # ---------------------------------------------------------------------------

    Write-Step "Creating dynamic security group: '$GroupDisplayName'"
    Write-Info "Membership rule: $MembershipRule"

    $groupParams = @{
        DisplayName                   = $GroupDisplayName
        MailEnabled                   = $false
        MailNickname                  = $GroupMailNickname
        SecurityEnabled               = $true
        GroupTypes                    = @("DynamicMembership")
        MembershipRule                = $MembershipRule
        MembershipRuleProcessingState = "On"
    }

    try {
        $newGroup = New-MgGroup -BodyParameter $groupParams -ErrorAction Stop
        $GroupObjectId = $newGroup.Id
        Write-Ok "Group created successfully"
        Write-Info "Object ID: $GroupObjectId"
    }
    catch {
        Write-Host ""
        Write-Host "   [FAIL] Failed to create group." -ForegroundColor Red
        Write-Host ""
        Write-Host "   Common causes:" -ForegroundColor Yellow
        Write-Host "     - Insufficient Entra ID permissions (need Groups Administrator or Global Admin)" -ForegroundColor White
        Write-Host "     - Dynamic groups require Entra ID P1 or P2 license" -ForegroundColor White
        Write-Host "     - Invalid membership rule syntax" -ForegroundColor White
        Write-Host ""
        Write-Error $_
        exit 1
    }
}

# ---------------------------------------------------------------------------
# 5. Summary
# ---------------------------------------------------------------------------

Write-Host ""
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host "  Group ready. Pass the Object ID to the setup script:" -ForegroundColor Cyan
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Group Name  : $GroupDisplayName"   -ForegroundColor White
Write-Host "  Object ID   : $GroupObjectId"       -ForegroundColor Green
Write-Host "  Rule        : $MembershipRule"       -ForegroundColor White
Write-Host ""
Write-Host "  NOTE: Dynamic membership can take several minutes to populate." -ForegroundColor Yellow
Write-Host "  Check members later with:"                                       -ForegroundColor Gray
Write-Host "    Get-MgGroupMember -GroupId '$GroupObjectId' -All"             -ForegroundColor White
Write-Host ""
Write-Host "  Next step -- run the Azure OpenAI setup:"                        -ForegroundColor Cyan
Write-Host "    .\setup-azure-openai.ps1 ``"                                   -ForegroundColor White
Write-Host "      -SubscriptionId     '<subscription-id>' ``"                  -ForegroundColor White
Write-Host "      -EntraGroupObjectId '$GroupObjectId'"                        -ForegroundColor White
Write-Host ""
Write-Host "================================================================" -ForegroundColor Cyan
Write-Host ""
