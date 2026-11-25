/// <reference types="vite/client" />

/** The git tag of the source this UI was built from (see vite.config.ts); empty when unknown. */
declare const __PEDRO_SOURCE_VERSION__: string

declare module '*.svg' {
  const content: string
  export default content
}
