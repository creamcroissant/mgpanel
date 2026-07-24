import { Switch, Input, Select, SelectContent, SelectItem, SelectTrigger, SelectValue, Button } from "@/components/ui";

export type CDNProvider = "cloudfront" | "cloudflare";

export type CDNConfig = {
  provider: CDNProvider;
  domain: string;
  originPath: string;
  originProtocol: "https" | "http";
  edges: number;
};

type CDNAccelerationEditorProps = {
  isXHTTPSelected: boolean;
  cdnConfig: CDNConfig;
  onConfigChange: (config: CDNConfig) => void;
  onDeploy: () => void;
};

const PROVIDER_OPTIONS: { value: CDNProvider; label: string }[] = [
  { value: "cloudfront", label: "CloudFront" },
  { value: "cloudflare", label: "Cloudflare" },
];

const PROTOCOL_OPTIONS: { value: "https" | "http"; label: string }[] = [
  { value: "https", label: "HTTPS" },
  { value: "http", label: "HTTP" },
];

const FIELD_IDS = {
  provider: "config-center-cdn-provider-trigger",
  providerLabel: "config-center-cdn-provider-label",
  domain: "config-center-cdn-domain-input",
  originPath: "config-center-cdn-origin-path-input",
  originProtocol: "config-center-cdn-origin-protocol-trigger",
  originProtocolLabel: "config-center-cdn-origin-protocol-label",
};

export function CDNAccelerationEditor({
  isXHTTPSelected,
  cdnConfig,
  onConfigChange,
  onDeploy,
}: CDNAccelerationEditorProps) {
  const enabled = cdnConfig.domain !== "" || cdnConfig.originPath !== "";

  const handleToggle = (checked: boolean) => {
    onConfigChange({
      ...cdnConfig,
      domain: checked ? "" : "",
      originPath: checked ? "/xborder/stream" : "",
    });
  };

  if (!isXHTTPSelected) {
    return null;
  }

  return (
    <div className="space-y-4 rounded-md border bg-muted/20 p-4" data-testid="config-center-cdn-editor">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <h3 className="text-sm font-semibold">CDN Acceleration</h3>
          <p className="text-xs text-muted-foreground">
            Enable CDN acceleration for XHTTP transport
          </p>
        </div>
        <Switch
          checked={enabled}
          onCheckedChange={handleToggle}
          aria-label="Toggle CDN acceleration"
          data-testid="config-center-cdn-toggle"
        />
      </div>

      {enabled && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <div className="space-y-2">
              <label id={FIELD_IDS.providerLabel} htmlFor={FIELD_IDS.provider} className="text-sm font-medium">
                Provider
              </label>
              <Select
                value={cdnConfig.provider}
                onValueChange={(value: string) =>
                  onConfigChange({ ...cdnConfig, provider: value as CDNProvider })
                }
              >
                <SelectTrigger
                  id={FIELD_IDS.provider}
                  aria-labelledby={FIELD_IDS.providerLabel}
                  data-testid="config-center-cdn-provider"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PROVIDER_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <label htmlFor={FIELD_IDS.domain} className="text-sm font-medium">
                CDN Domain
              </label>
              <Input
                id={FIELD_IDS.domain}
                data-testid="config-center-cdn-domain"
                value={cdnConfig.domain}
                onChange={(e) =>
                  onConfigChange({ ...cdnConfig, domain: e.target.value })
                }
                placeholder="cdn.example.com"
              />
            </div>
            <div className="space-y-2">
              <label htmlFor={FIELD_IDS.originPath} className="text-sm font-medium">
                Origin Path
              </label>
              <Input
                id={FIELD_IDS.originPath}
                data-testid="config-center-cdn-origin-path"
                value={cdnConfig.originPath}
                onChange={(e) =>
                  onConfigChange({ ...cdnConfig, originPath: e.target.value })
                }
                placeholder="/xborder/stream"
              />
            </div>
            <div className="space-y-2">
              <label id={FIELD_IDS.originProtocolLabel} htmlFor={FIELD_IDS.originProtocol} className="text-sm font-medium">
                Origin Protocol
              </label>
              <Select
                value={cdnConfig.originProtocol}
                onValueChange={(value: string) =>
                  onConfigChange({ ...cdnConfig, originProtocol: value as "https" | "http" })
                }
              >
                <SelectTrigger
                  id={FIELD_IDS.originProtocol}
                  aria-labelledby={FIELD_IDS.originProtocolLabel}
                  data-testid="config-center-cdn-origin-protocol"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PROTOCOL_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>
                      {opt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="flex items-center justify-between rounded-md border bg-background px-3 py-2">
            <div className="space-y-0.5">
              <p className="text-sm font-medium">Edge Count</p>
              <p className="text-xs text-muted-foreground">{cdnConfig.edges} edge locations</p>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  onConfigChange({
                    ...cdnConfig,
                    edges: Math.max(1, cdnConfig.edges - 1),
                  })
                }
                aria-label="Decrease edge count"
                data-testid="config-center-cdn-edges-dec"
              >
                -
              </Button>
              <span className="w-8 text-center text-sm tabular-nums">{cdnConfig.edges}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={() =>
                  onConfigChange({
                    ...cdnConfig,
                    edges: cdnConfig.edges + 1,
                  })
                }
                aria-label="Increase edge count"
                data-testid="config-center-cdn-edges-inc"
              >
                +
              </Button>
            </div>
          </div>

          <Button
            onClick={onDeploy}
            data-testid="config-center-cdn-deploy"
          >
            Deploy CDN Config
          </Button>
        </div>
      )}
    </div>
  );
}
