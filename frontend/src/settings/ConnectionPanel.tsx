import { Cloud, KeyRound, LogIn, LogOut, Plug, Server, Sparkles, Zap } from 'lucide-react'
import { Field, FieldLabel } from '@ui-sdk/react/field'
import { Button } from '../ui/button'
import { SecretField, TextField } from '../ui/fields'
import { AppSelect, type SelectOption } from '../ui/select'
import {
  DEFAULT_IMAGE_DEPLOYMENT,
  DEFAULT_TRANSCRIPTION_DEPLOYMENT,
  formatCheckedAt,
} from './settingsModel'
import type { SettingsDraftState } from './useSettingsDraft'

const PROVIDERS: SelectOption[] = [
  { value: 'azure', label: 'Azure OpenAI', description: 'Sign in with Microsoft Entra ID', icon: <Cloud aria-hidden="true" /> },
  { value: 'azure_apikey', label: 'Azure OpenAI (API key)', description: 'Key-based access, image & audio models', icon: <KeyRound aria-hidden="true" /> },
  { value: 'openai', label: 'OpenAI', description: 'api.openai.com with your API key', icon: <Sparkles aria-hidden="true" /> },
  { value: 'compat', label: 'Local / OpenAI-compatible', description: 'LM Studio, Ollama, llama.cpp…', icon: <Server aria-hidden="true" /> },
]

function ConnectionStatus({ s }: { s: SettingsDraftState }) {
  const { connection, hydrated, dirty } = s
  const tone = connection.kind === 'ok' ? 'success' : connection.kind === 'fail' ? 'danger' : undefined
  const label = !hydrated
    ? 'Loading…'
    : connection.kind === 'ok'
      ? 'Connected'
      : connection.kind === 'fail'
        ? 'Connection failed'
        : dirty
          ? 'Unsaved changes'
          : 'Not tested yet'
  const detail =
    connection.kind === 'ok'
      ? `Verified at ${formatCheckedAt(connection.checkedAt)}`
      : connection.kind === 'fail'
        ? connection.message
        : 'Run a connection test to verify these settings.'

  return (
    <div className="connection-card" data-tone={tone}>
      <span className="status-dot" data-tone={tone} aria-hidden="true" />
      <div className="connection-card-text">
        <strong>{label}</strong>
        <span title={detail}>{detail}</span>
      </div>
    </div>
  )
}

export function ConnectionPanel({ s }: { s: SettingsDraftState }) {
  const { draft, update } = s
  const pt = draft.providerType
  const isAzure = pt === 'azure' || pt === 'azure_apikey'

  return (
    <div className="settings-stack">
      <Field>
        <FieldLabel>Provider</FieldLabel>
        <AppSelect value={pt} onValueChange={v => update({ providerType: v })} options={PROVIDERS} />
      </Field>

      <ConnectionStatus s={s} />

      <div className="settings-card">
        <div className="settings-card-head">
          <Plug aria-hidden="true" />
          <span>Endpoint</span>
        </div>
        <div className="settings-grid">
          {isAzure ? (
            <>
              <TextField
                className="span-2"
                label="Resource URL"
                description="API base URL of your Azure OpenAI resource."
                value={draft.endpoint}
                onValueChange={v => update({ endpoint: v })}
                placeholder="https://your-resource.openai.azure.com"
                mono
              />
              <TextField
                label="Deployment"
                value={draft.deployment}
                onValueChange={v => update({ deployment: v })}
                placeholder="gpt-4"
                mono
              />
            </>
          ) : null}

          {pt === 'azure' ? (
            <TextField
              label="Directory (tenant) ID"
              optional
              description="Only if your organization needs a specific tenant."
              value={draft.azureTenantId}
              onValueChange={v => update({ azureTenantId: v })}
              placeholder="contoso.onmicrosoft.com"
              mono
            />
          ) : null}

          {pt === 'azure_apikey' ? (
            <>
              <SecretField
                label="API key"
                value={draft.azureApiKey}
                onValueChange={v => update({ azureApiKey: v })}
                placeholder="Azure OpenAI API key"
              />
              <TextField
                label="Image deployment"
                optional
                description="Used by generate_image. Leave empty to turn it off."
                value={draft.imageDeployment}
                onValueChange={v => update({ imageDeployment: v })}
                placeholder={DEFAULT_IMAGE_DEPLOYMENT}
                mono
              />
              <TextField
                label="Transcription deployment"
                optional
                description="Transcribes attached audio. Leave empty to turn it off."
                value={draft.transcriptionDeployment}
                onValueChange={v => update({ transcriptionDeployment: v })}
                placeholder={DEFAULT_TRANSCRIPTION_DEPLOYMENT}
                mono
              />
            </>
          ) : null}

          {pt === 'openai' ? (
            <>
              <SecretField label="API key" value={draft.apiKey} onValueChange={v => update({ apiKey: v })} placeholder="sk-…" />
              <TextField label="Model" value={draft.model} onValueChange={v => update({ model: v })} placeholder="gpt-4o" mono />
            </>
          ) : null}

          {pt === 'compat' ? (
            <>
              <TextField
                className="span-2"
                label="Base URL"
                description="LM Studio :1234/v1 · llama.cpp :8080/v1 · Ollama :11434/v1"
                value={draft.compatBaseURL}
                onValueChange={v => update({ compatBaseURL: v })}
                placeholder="http://localhost:1234/v1"
                mono
              />
              <SecretField
                label="API key"
                optional
                description="Most local servers don't need one."
                value={draft.compatApiKey}
                onValueChange={v => update({ compatApiKey: v })}
                placeholder="Not required"
              />
              <TextField
                label="Model"
                value={draft.compatModel}
                onValueChange={v => update({ compatModel: v })}
                placeholder="llama-3.2-8b-instruct"
                mono
              />
            </>
          ) : null}
        </div>
      </div>

      <div className="settings-card settings-auth">
        <div className="settings-auth-status">
          <span
            className="status-dot"
            data-tone={pt === 'compat' || s.authenticated ? 'success' : undefined}
            aria-hidden="true"
          />
          <div>
            <strong>
              {pt === 'compat' ? 'Local server' : s.authenticated ? 'Signed in' : 'Not signed in'}
            </strong>
            <span className="settings-muted">
              {pt === 'compat'
                ? 'No sign-in required.'
                : pt === 'azure'
                  ? 'Pedro opens your browser to sign in with Azure.'
                  : 'Authenticated with the API key above.'}
            </span>
          </div>
        </div>
        <div className="settings-auth-actions">
          {pt === 'azure' ? (
            s.authenticated ? (
              <Button variant="secondary" size="sm" onClick={() => s.signOut()}>
                <LogOut aria-hidden="true" />
                Sign out
              </Button>
            ) : (
              <Button variant="secondary" size="sm" onClick={() => s.signIn()} pendingAnnouncement="Opening browser">
                <LogIn aria-hidden="true" />
                Sign in with Azure
              </Button>
            )
          ) : null}
          <Button variant="primary" size="sm" disabled={!s.hydrated} onClick={() => s.test()} pendingAnnouncement="Testing connection">
            <Zap aria-hidden="true" />
            {s.dirty ? 'Save & test' : 'Test connection'}
          </Button>
        </div>
      </div>
    </div>
  )
}
