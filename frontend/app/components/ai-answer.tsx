import React, { useState } from 'react';
import { Sparkles, Info, Copy, Check, ExternalLink } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import { Citation } from './api-client';
import { Button } from './ui/button';

interface AIAnswerProps {
  answer: string;
  citations?: Citation[];
  lastUpdated?: number;
}

export default function AIAnswer({ answer, citations, lastUpdated }: AIAnswerProps) {
  const [copied, setCopied] = useState(false);

  // Process citations
  const processMarkdownWithCitations = (markdown: string): string => {
    if (!citations || citations.length === 0) return markdown;
    
    const regex = /\[(\d+)\]/g;
    
    return markdown.replace(regex, (match, index) => {
      const citationIndex = parseInt(index, 10);
      const citation = citations.find(c => c.index === citationIndex);
      
      if (citation) {
        return `<sup>[${index}](#citation-${index})</sup>`;
      }
      
      return match;
    });
  };

  // Format time
  const formatLastUpdated = () => {
    if (!lastUpdated) return null;
    const date = new Date(lastUpdated * 1000);
    return date.toLocaleString();
  };

  // Handle copy
  const handleCopy = async () => {
    await navigator.clipboard.writeText(answer);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  // Custom React Markdown components
  const components = {
    a: (props: any) => {
      const isCitation = props.href?.startsWith('#citation-');
      
      if (isCitation && citations) {
        const citationId = props.href.replace('#citation-', '');
        const citation = citations.find(c => c.index === parseInt(citationId, 10));
        
        if (citation) {
          return (
            <a
              href={citation.url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
              title={`${citation.title} (r/${citation.subreddit})`}
            >
              {props.children}
            </a>
          );
        }
      }
      
      return (
        <a
          href={props.href}
          target="_blank"
          rel="noopener noreferrer"
          className="text-blue-600 dark:text-blue-400 hover:underline inline-flex items-center"
        >
          {props.children}
          <ExternalLink className="ml-1 h-3 w-3 inline-block" />
        </a>
      );
    },
    sup: (props: any) => (
      <sup 
        className="text-xs bg-primary/10 text-primary px-1.5 py-0.5 rounded-full"
      >
        {props.children}
      </sup>
    ),
    h1: (props: any) => <h1 className="text-xl font-bold mt-6 mb-2">{props.children}</h1>,
    h2: (props: any) => <h2 className="text-lg font-bold mt-5 mb-2">{props.children}</h2>,
    h3: (props: any) => <h3 className="text-base font-semibold mt-4 mb-2">{props.children}</h3>,
    p: (props: any) => <p className="mb-4 leading-relaxed">{props.children}</p>,
    ul: (props: any) => <ul className="mb-4 pl-5 list-disc">{props.children}</ul>,
    ol: (props: any) => <ol className="mb-4 pl-5 list-decimal">{props.children}</ol>,
    li: (props: any) => <li className="mb-1">{props.children}</li>,
    blockquote: (props: any) => (
      <blockquote 
        className="border-l-4 border-primary/30 pl-4 italic my-4 text-neutral-700 dark:text-neutral-300" 
      >
        {props.children}
      </blockquote>
    ),
    code: (props: any) => {
      const { inline } = props;
      return inline ? (
        <code 
          className="bg-neutral-100 dark:bg-neutral-800 px-1.5 py-0.5 rounded text-sm font-mono"
        >
          {props.children}
        </code>
      ) : (
        <code 
          className="block bg-neutral-100 dark:bg-neutral-800 p-3 my-4 rounded-md overflow-x-auto text-sm font-mono"
        >
          {props.children}
        </code>
      );
    }
  };

  return (
    <div className="mt-5">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Sparkles className="size-5 text-primary" />
          <h2 className="text-base font-semibold text-neutral-800 dark:text-neutral-200">
            Answer
          </h2>
        </div>
        
        <div className="flex items-center gap-2">
          {lastUpdated && (
            <div className="flex items-center gap-1.5 text-xs text-neutral-500 dark:text-neutral-400 mr-2">
              <Info className="size-3" />
              <span>As of {formatLastUpdated()}</span>
            </div>
          )}
          
          <Button 
            variant="ghost" 
            size="sm" 
            onClick={handleCopy}
            className="h-8 px-2 text-xs rounded-full"
          >
            {copied ? (
              <Check className="h-3.5 w-3.5 mr-1.5" />
            ) : (
              <Copy className="h-3.5 w-3.5 mr-1.5" />
            )}
            {copied ? "Copied" : "Copy"}
          </Button>
        </div>
      </div>
      
      <div className="prose prose-sm max-w-none dark:prose-invert pb-2">
        <ReactMarkdown components={components}>
          {processMarkdownWithCitations(answer)}
        </ReactMarkdown>
      </div>
    </div>
  );
}