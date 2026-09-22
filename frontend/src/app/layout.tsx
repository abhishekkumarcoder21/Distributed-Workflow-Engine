import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Distributed Workflow Engine — Control Plane",
  description: "Real-time monitoring and orchestration dashboard for distributed background jobs, workers, queues, and DAG workflows.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
