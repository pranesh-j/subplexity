// frontend/app/components/ai-answer.tsx
import React from 'react';
import { Sparkles, Info } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import { Citation } from './api-client';

interface AIAnswerProps {
  answer: string;
  citations?: Citation[];
  lastUpdated?: number;
}

export default function AIAnswer({ answer, citations, lastUpdated }: AIAnswerProps) {
  // Function to insert citation links in markdown
  const processMarkdownWithCitations = (markdown: string): string => {
    if (!citations || citations.length === 0) return markdown;

    // Replace citation patterns like [1] with markdown links
    let processed = markdown;
    
    // This regex looks for citation indices in square brackets
    const regex = /\[(\d+)\]/g;
    
    processed = processed.replace(regex, (match, index) => {
      const citationIndex = parseInt(index, 10);
      const citation = citations.find(c => c.index === citationIndex);
      
      if (citation) {
        // Return a superscript with a link
        return `<sup>[${index}](#citation-${index})</sup>`;
      }
      
      return match; // Keep as is if no matching citation
    });
    
    return processed;
  };

  // Format the last updated time
  const formatLastUpdated = () => {
    if (!lastUpdated) return null;
    
    const date = new Date(lastUpdated * 1000);
    return date.toLocaleString();
  };

  return (
    <div className="mt-5">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Sparkles className="size-5 text-primary" />
          <h2 className="text-base font-semibold text-neutral-800 dark:text-neutral-200">
            Answer
          </h2>
        </div>
        
        {lastUpdated && (
          <div className="flex items-center gap-1.5 text-xs text-neutral-500 dark:text-neutral-400">
            <Info className="size-3" />
            <span>As of {formatLastUpdated()}</span>
          </div>
        )}
      </div>
      
      <div className="prose prose-sm max-w-none dark:prose-invert pb-2">
        <ReactMarkdown>{processMarkdownWithCitations(answer)}</ReactMarkdown>
      </div>
    </div>
  );
}