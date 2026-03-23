import { useState, useEffect, useRef, useCallback } from 'react'

interface VoiceControlsProps {
  onSend: (content: string) => void
  autoSpeak: boolean
  setAutoSpeak: (val: boolean) => void
  ttsSpeed: number
  setTtsSpeed: (val: number) => void
  ttsPitch: number
  setTtsPitch: (val: number) => void
}

interface PendingMessage {
  text: string
  timestamp: number
}

export default function VoiceControls({ 
  onSend, 
  autoSpeak, 
  setAutoSpeak,
  ttsSpeed, 
  setTtsSpeed,
  ttsPitch, 
  setTtsPitch 
}: VoiceControlsProps) {
  const [listening, setListening] = useState(false)
  const [showVoiceSettings, setShowVoiceSettings] = useState(false)
  const synthRef = useRef<SpeechSynthesis | null>(null)
  const recognitionRef = useRef<SpeechRecognition | null>(null)
  const pendingMessagesRef = useRef<PendingMessage[]>([])

  useEffect(() => {
    synthRef.current = window.speechSynthesis
    const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition
    if (SpeechRecognition) {
      recognitionRef.current = new SpeechRecognition()
      recognitionRef.current.continuous = false
      recognitionRef.current.interimResults = true
      
      recognitionRef.current.onresult = (event) => {
        const transcript = Array.from(event.results)
          .map(result => result[0].transcript)
          .join('')
        
        if (event.results[event.results.length - 1].isFinal) {
          if (transcript.trim()) {
            onSend(transcript.trim())
          }
          setListening(false)
        }
      }

      recognitionRef.current.onerror = () => {
        setListening(false)
      }

      recognitionRef.current.onend = () => {
        setListening(false)
      }
    }
  }, [onSend])

  useEffect(() => {
    pendingMessagesRef.current.push({ text: '', timestamp: Date.now() })
  }, [])

  const speak = useCallback((text: string) => {
    if (!synthRef.current || !autoSpeak) return
    
    synthRef.current.cancel()
    const utterance = new SpeechSynthesisUtterance(text)
    utterance.rate = ttsSpeed
    utterance.pitch = ttsPitch
    synthRef.current.speak(utterance)
  }, [autoSpeak, ttsSpeed, ttsPitch])

  const startListening = () => {
    if (recognitionRef.current) {
      setListening(true)
      recognitionRef.current.start()
    }
  }

  const stopListening = () => {
    if (recognitionRef.current) {
      recognitionRef.current.stop()
      setListening(false)
    }
  }

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.ctrlKey && e.key === 'v' && !listening) {
        const items = e.clipboardData?.items
        if (items) {
          for (const item of Array.from(items)) {
            if (item.type.startsWith('image/')) {
              return
            }
          }
        }
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [listening])

  return {
    listening,
    startListening,
    stopListening,
    speak,
    autoSpeak,
    setAutoSpeak,
    ttsSpeed,
    setTtsSpeed,
    ttsPitch,
    setTtsPitch,
    showVoiceSettings,
    setShowVoiceSettings
  }
}

declare global {
  interface Window {
    SpeechRecognition: typeof SpeechRecognition
    webkitSpeechRecognition: typeof SpeechRecognition
  }
}

export { useState, useEffect, useRef, useCallback }