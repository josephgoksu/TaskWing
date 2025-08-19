import { useState, useEffect, useRef } from 'react'
import './TypewriterTerminal.css'

interface TerminalLine {
  type: 'prompt' | 'command' | 'output'
  text: string
  delay?: number
}

interface TerminalSequence {
  lines: TerminalLine[]
  pauseAfter?: number
}

interface TypewriterTerminalProps {
  sequences: TerminalSequence[]
  title?: string
  className?: string
}

export function TypewriterTerminal({ sequences, title = 'taskwing', className = '' }: TypewriterTerminalProps) {
  const [currentSequence, setCurrentSequence] = useState(0)
  const [currentLine, setCurrentLine] = useState(0)
  const [currentChar, setCurrentChar] = useState(0)
  const [displayedLines, setDisplayedLines] = useState<string[]>([])
  const [isTyping, setIsTyping] = useState(false)
  const timeoutRef = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (sequences.length === 0) return

    const currentSeq = sequences[currentSequence]
    const line = currentSeq.lines[currentLine]

    if (!line) {
      // Sequence complete, pause and move to next
      timeoutRef.current = window.setTimeout(() => {
        setCurrentSequence((prev) => (prev + 1) % sequences.length)
        setCurrentLine(0)
        setCurrentChar(0)
        setDisplayedLines([])
      }, currentSeq.pauseAfter || 2000)
      return
    }

    if (currentChar === 0) {
      setIsTyping(true)
      // Add new line to display
      setDisplayedLines(prev => [...prev, ''])
    }

    if (currentChar < line.text.length) {
      const typingSpeed = line.type === 'command' ? 100 : 50
      const delay = line.delay || typingSpeed

      timeoutRef.current = window.setTimeout(() => {
        setDisplayedLines(prev => {
          const newLines = [...prev]
          newLines[newLines.length - 1] = line.text.slice(0, currentChar + 1)
          return newLines
        })
        setCurrentChar(prev => prev + 1)
      }, delay)
    } else {
      // Line complete
      setIsTyping(false)
      timeoutRef.current = window.setTimeout(() => {
        setCurrentLine(prev => prev + 1)
        setCurrentChar(0)
      }, line.type === 'command' ? 500 : 200)
    }

    return () => {
      if (timeoutRef.current) {
        window.clearTimeout(timeoutRef.current)
      }
    }
  }, [sequences, currentSequence, currentLine, currentChar])

  const renderLine = (text: string, index: number) => {
    const line = sequences[currentSequence]?.lines[index]
    const isCurrentLine = index === displayedLines.length - 1 && isTyping

    if (line?.type === 'prompt') {
      return (
        <div key={index} className="terminal-line">
          <span className="prompt">$</span>
          <span className="command">{text}</span>
          {isCurrentLine && <span className="cursor">_</span>}
        </div>
      )
    } else if (line?.type === 'command') {
      return (
        <div key={index} className="terminal-line">
          <span className="prompt">$</span>
          <span className="command">{text}</span>
          {isCurrentLine && <span className="cursor">_</span>}
        </div>
      )
    } else {
      return (
        <div key={index} className="terminal-line">
          <span className="output">{text}</span>
          {isCurrentLine && <span className="cursor">_</span>}
        </div>
      )
    }
  }

  return (
    <div className={`typewriter-terminal ${className}`}>
      <div className="terminal-header">
        <div className="terminal-buttons">
          <span className="btn red"></span>
          <span className="btn yellow"></span>
          <span className="btn green"></span>
        </div>
        <span className="terminal-title">{title}</span>
      </div>
      <div className="terminal-body">
        {displayedLines.map((text, index) => renderLine(text, index))}
      </div>
    </div>
  )
}