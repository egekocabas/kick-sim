import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getSimulatorKeyInfo, getSimulatorPublicKey, rotateSimulatorKey } from "../../api/generated/client";
import type { KeyInfo } from "../../api/generated/models";
import { Code, Info, Notice, QueryState, Section } from "../../components/ui";
import { errorMessage, successful } from "../../lib/api";
import { queryKeys } from "../../lib/queryKeys";

export function KeysPage() {
  const queryClient = useQueryClient();
  const key = useQuery({ queryKey: queryKeys.key, queryFn: async () => successful<KeyInfo>(await getSimulatorKeyInfo()) });
  const publicKey = useQuery({ queryKey: queryKeys.publicKey, queryFn: async () => successful<{ path: string; pem: string }>(await getSimulatorPublicKey()) });
  const rotate = useMutation({
    mutationFn: async () => successful(await rotateSimulatorKey({ headers: { "Content-Type": "application/json" } })),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.key }),
        queryClient.invalidateQueries({ queryKey: queryKeys.publicKey }),
      ]);
    },
  });
  const queryError = key.error ?? publicKey.error;
  return (
    <div className="page-content">
      <Section title="Simulator signing key" hint="The private key stays in the local workspace">
        <QueryState pending={key.isPending || publicKey.isPending} error={queryError} />
        <div className="detail-grid"><Info label="Algorithm" value={key.data ? `${key.data.algorithm} ${key.data.bits}` : "…"} /><Info label="Fingerprint" value={key.data?.fingerprint ?? "…"} /><Info label="Pair status" value={key.data?.matchingPrivateKey ? "Public/private keys match" : "Key mismatch"} /><Info label="Public key path" value={key.data?.path ?? "…"} /></div>
        <Code value={publicKey.data?.pem ?? "Loading public key…"} />
        <button className="danger" disabled={rotate.isPending} onClick={() => { if (window.confirm("Rotate the simulator key pair? Existing receiver setups will need the new public key.")) rotate.mutate(); }} type="button">{rotate.isPending ? "Rotating…" : "Rotate key pair"}</button>
        {rotate.error && <Notice tone="error">{errorMessage(rotate.error)}</Notice>}
      </Section>
    </div>
  );
}
