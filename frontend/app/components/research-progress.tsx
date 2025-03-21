// frontend/app/components/research-progress.tsx
import React from 'react';
import { Check, Loader2 } from 'lucide-react';

export interface ResearchStep {
  title: string;
  status: 'pending' | 'in-progress' | 'complete';
  details?: string;
}

interface ResearchProgressProps {
  steps: ResearchStep[];
  currentStep: number;
}

export function ResearchProgress({ steps, currentStep }: ResearchProgressProps) {
  return (
    <div className="mt-4 border border-neutral-200 dark:border-neutral-800 rounded-xl p-4 bg-white dark:bg-neutral-900/30">
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm font-medium">Research Progress</span>
        <span className="text-xs text-blue-400 bg-blue-900/30 px-2 py-0.5 rounded-full">
          In Progress
        </span>
      </div>
      
      <div className="space-y-3">
        {steps.map((step, index) => (
          <div 
            key={index}
            className={`flex items-center gap-3 p-2 rounded ${
              currentStep === index ? 'bg-neutral-100 dark:bg-neutral-800/50' : ''
            }`}
          >
            <div className="w-6 h-6 rounded-full flex items-center justify-center bg-neutral-100 dark:bg-neutral-800">
              {step.status === 'complete' ? (
                <Check className="w-3 h-3 text-green-500" />
              ) : step.status === 'in-progress' ? (
                <Loader2 className="w-3 h-3 text-blue-400 animate-spin" />
              ) : (
                <div className="w-2 h-2 rounded-full bg-neutral-300 dark:bg-neutral-600" />
              )}
            </div>
            <div>
              <span className={`text-sm ${
                step.status === 'complete' 
                  ? 'text-green-500' 
                  : step.status === 'in-progress'
                  ? 'text-blue-400'
                  : 'text-neutral-400'
              }`}>
                {step.title}
              </span>
              {step.details && (
                <p className="text-xs text-neutral-500 mt-0.5">{step.details}</p>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}