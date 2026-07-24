import { useState, useEffect, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Trash2, XCircle, Copy, CheckCircle2 } from "lucide-react";
import {
  fetchMCPKeys,
  createMCPKey,
  revokeMCPKey,
  deleteMCPKey,
  type MCPApiKey,
} from "@/api/admin";
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  Input,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Badge,
} from "@/components/ui";

export default function MCPKeysPage() {
  const { t } = useTranslation();
  const [keys, setKeys] = useState<MCPApiKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [createName, setCreateName] = useState("");
  const [newKeyValue, setNewKeyValue] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const loadKeys = useCallback(async () => {
    setLoading(true);
    try {
      const data = await fetchMCPKeys();
      setKeys(data);
    } catch (err) {
      console.error("Failed to load MCP keys", err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadKeys();
  }, [loadKeys]);

  const handleCreate = async () => {
    if (!createName.trim()) return;
    try {
      const result = await createMCPKey({ name: createName.trim() });
      if (result.key) {
        setNewKeyValue(result.key);
      }
      setCreateName("");
      await loadKeys();
    } catch (err) {
      console.error("Failed to create MCP key", err);
    }
  };

  const handleRevoke = async (id: number) => {
    try {
      await revokeMCPKey(id);
      await loadKeys();
    } catch (err) {
      console.error("Failed to revoke MCP key", err);
    }
  };

  const handleDelete = async (id: number) => {
    if (!confirm(t("admin.mcpKeys.confirmDelete"))) return;
    try {
      await deleteMCPKey(id);
      await loadKeys();
    } catch (err) {
      console.error("Failed to delete MCP key", err);
    }
  };

  const handleCopyKey = () => {
    if (newKeyValue) {
      navigator.clipboard.writeText(newKeyValue);
      setCopied(true);
      setTimeout(() => setCopied(false), 3000);
    }
  };

  const formatDate = (ts: number) => {
    if (!ts) return "-";
    return new Date(ts * 1000).toLocaleString();
  };

  if (loading) {
    return <div className="p-8 text-center">{t("common.loading")}</div>;
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t("admin.mcpKeys.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("admin.mcpKeys.description")}</p>
        </div>
        <Dialog
          open={!!newKeyValue}
          onOpenChange={(open) => { if (!open) { setNewKeyValue(null); setCopied(false); } }}
        >
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              {t("admin.mcpKeys.create")}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("admin.mcpKeys.createTitle")}</DialogTitle>
              <DialogDescription>{t("admin.mcpKeys.createDescription")}</DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <label className="text-sm font-medium" htmlFor="key-name">{t("admin.mcpKeys.keyName")}</label>
                <Input
                  id="key-name"
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder={t("admin.mcpKeys.keyNamePlaceholder")}
                />
              </div>
              <Button onClick={handleCreate} disabled={!createName.trim()}>
                {t("admin.mcpKeys.generate")}
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {/* New key reveal dialog */}
      <Dialog open={!!newKeyValue && newKeyValue.length > 0} onOpenChange={(open) => { if (!open) { setNewKeyValue(null); setCopied(false); } }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("admin.mcpKeys.keyCreated")}</DialogTitle>
            <DialogDescription>{t("admin.mcpKeys.keyCreatedDescription")}</DialogDescription>
          </DialogHeader>
          <div className="flex items-center space-x-2">
            <div className="grid flex-1 gap-2">
              <label htmlFor="new-key" className="sr-only">{t("admin.mcpKeys.newKey")}</label>
              <Input id="new-key" value={newKeyValue || ""} readOnly className="font-mono text-sm" />
            </div>
            <Button type="submit" size="sm" className="px-3" onClick={handleCopyKey}>
              {copied ? <CheckCircle2 className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
            </Button>
          </div>
          <DialogFooter className="sm:justify-start">
            <Button type="button" variant="secondary" onClick={() => { setNewKeyValue(null); }}>
              {t("common.close")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Card>
        <CardHeader>
          <CardTitle>{t("admin.mcpKeys.listTitle")}</CardTitle>
          <CardDescription>{t("admin.mcpKeys.listDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("admin.mcpKeys.name")}</TableHead>
                <TableHead>{t("admin.mcpKeys.prefix")}</TableHead>
                <TableHead>{t("admin.mcpKeys.status")}</TableHead>
                <TableHead>{t("admin.mcpKeys.lastUsed")}</TableHead>
                <TableHead>{t("admin.mcpKeys.created")}</TableHead>
                <TableHead className="text-right">{t("admin.mcpKeys.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {keys.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground">
                    {t("admin.mcpKeys.noKeys")}
                  </TableCell>
                </TableRow>
              ) : (
                keys.map((key) => (
                  <TableRow key={key.id}>
                    <TableCell className="font-medium">{key.name || "-"}</TableCell>
                    <TableCell className="font-mono text-sm">{key.prefix}...</TableCell>
                    <TableCell>
                      <Badge variant={key.enabled ? "default" : "secondary"}>
                        {key.enabled ? t("admin.mcpKeys.enabled") : t("admin.mcpKeys.disabled")}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-sm">{formatDate(key.last_used_at ?? 0)}</TableCell>
                    <TableCell className="text-sm">{formatDate(key.created_at)}</TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-2">
                        {key.enabled && (
                          <Button variant="outline" size="icon" onClick={() => handleRevoke(key.id)} title={t("admin.mcpKeys.revoke")}>
                            <XCircle className="h-4 w-4" />
                          </Button>
                        )}
                        <Button variant="destructive" size="icon" onClick={() => handleDelete(key.id)} title={t("admin.mcpKeys.delete")}>
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
