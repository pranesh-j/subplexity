"use client"

import * as React from "react"
import { cn } from "@/lib/utils"

export interface TextareaAutosizeProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  minRows?: number
  maxRows?: number
}

const TextareaAutosize = React.forwardRef<HTMLTextAreaElement, TextareaAutosizeProps>(
  ({ className, minRows = 1, maxRows = 5, onChange, ...props }, ref) => {
    const textareaRef = React.useRef<HTMLTextAreaElement | null>(null)
    const [textareaLineHeight, setTextareaLineHeight] = React.useState(20) // Default line height

    React.useEffect(() => {
      if (textareaRef.current) {
        // Get line height from computed styles
        const lineHeight = Number.parseInt(window.getComputedStyle(textareaRef.current).lineHeight, 10)
        if (!isNaN(lineHeight)) {
          setTextareaLineHeight(lineHeight)
        }
      }
    }, [])

    const handleChange = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
      const textarea = event.currentTarget

      // Reset height to auto to get the correct scrollHeight
      textarea.style.height = "auto"

      // Calculate new height based on scrollHeight
      const newHeight = Math.min(
        Math.max(textarea.scrollHeight, minRows * textareaLineHeight),
        maxRows * textareaLineHeight,
      )

      textarea.style.height = `${newHeight}px`

      if (onChange) {
        onChange(event)
      }
    }

    return (
      <textarea
        className={cn(
          "flex w-full resize-none bg-transparent text-sm placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50",
          className,
        )}
        ref={(element) => {
          // Assign to both refs
          textareaRef.current = element
          if (typeof ref === "function") {
            ref(element)
          } else if (ref) {
            ref.current = element
          }
        }}
        onChange={handleChange}
        rows={minRows}
        {...props}
      />
    )
  },
)

TextareaAutosize.displayName = "TextareaAutosize"

export { TextareaAutosize }