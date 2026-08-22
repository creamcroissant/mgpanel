"use client";

import { Toaster, type ToasterProps } from "sonner";

const defaultToastOptions: ToasterProps["toastOptions"] = {
  classNames: {
    toast:
      "group toast group-[.toaster]:bg-card group-[.toaster]:text-foreground group-[.toaster]:border-border group-[.toaster]:shadow-md",
    description: "group-[.toast]:text-muted-foreground",
    actionButton:
      "group-[.toast]:bg-primary group-[.toast]:text-primary-foreground",
    cancelButton: "group-[.toast]:bg-muted group-[.toast]:text-muted-foreground",
  },
};

export default function SonnerToaster(props: ToasterProps) {
  return (
    <Toaster
      position="top-right"
      closeButton
      duration={4500}
      toastOptions={defaultToastOptions}
      {...props}
    />
  );
}
