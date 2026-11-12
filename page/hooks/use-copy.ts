"use client";

import { useCallback } from "react";
import { useToast } from "@/hooks/use-toast";

// Copies text to the clipboard, falling back to execCommand on plain HTTP
// where the async clipboard API is unavailable.
export function useCopy() {
  const { toast } = useToast();
  return useCallback(
    async (text: string) => {
      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text);
        } else {
          const ta = document.createElement("textarea");
          ta.value = text;
          ta.style.position = "fixed";
          ta.style.opacity = "0";
          document.body.appendChild(ta);
          ta.select();
          const ok = document.execCommand("copy");
          document.body.removeChild(ta);
          if (!ok) throw new Error("execCommand failed");
        }
        toast({ title: "Copied", description: text });
      } catch {
        toast({ title: "Copy failed", description: text, variant: "destructive" });
      }
    },
    [toast],
  );
}
