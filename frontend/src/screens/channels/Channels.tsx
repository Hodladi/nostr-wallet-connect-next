import {
  ArrowDown,
  ArrowUp,
  Bitcoin,
  ChevronDown,
  CopyIcon,
  Hotel,
  MoreHorizontal,
  Trash2
} from "lucide-react";
import React from "react";
import { Link, useNavigate } from "react-router-dom";
import AppHeader from "src/components/AppHeader.tsx";
import Loading from "src/components/Loading.tsx";
import { Badge } from "src/components/ui/badge.tsx";
import { Button } from "src/components/ui/button.tsx";
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from "src/components/ui/card.tsx";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "src/components/ui/dropdown-menu.tsx";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "src/components/ui/table.tsx";
import { toast } from "src/components/ui/use-toast.ts";
import { ONCHAIN_DUST_SATS } from "src/constants.ts";
import { useAlbyBalance } from "src/hooks/useAlbyBalance.ts";
import { useBalances } from "src/hooks/useBalances.ts";
import { useChannels } from "src/hooks/useChannels";
import { useInfo } from "src/hooks/useInfo";
import { useNodeConnectionInfo } from "src/hooks/useNodeConnectionInfo.ts";
import { useRedeemOnchainFunds } from "src/hooks/useRedeemOnchainFunds.ts";
import { copyToClipboard } from "src/lib/clipboard.ts";
import { CloseChannelResponse, Node } from "src/types";
import { request } from "src/utils/request";
import { useCSRF } from "../../hooks/useCSRF.ts";

export default function Channels() {
  const { data: channels, mutate: reloadChannels } = useChannels();
  const { data: nodeConnectionInfo } = useNodeConnectionInfo();
  const { data: balances } = useBalances();
  const { data: albyBalance } = useAlbyBalance();
  const [nodes, setNodes] = React.useState<Node[]>([]);
  const { data: info, mutate: reloadInfo } = useInfo();
  const { data: csrf } = useCSRF();
  const navigate = useNavigate();
  const redeemOnchainFunds = useRedeemOnchainFunds();

  React.useEffect(() => {
    if (!info || info.running) {
      return;
    }
    navigate("/");
  }, [info, navigate]);

  const loadNodeStats = React.useCallback(async () => {
    if (!channels) {
      return [];
    }
    const nodes = await Promise.all(
      channels?.map(async (channel): Promise<Node | undefined> => {
        try {
          const response = await request<Node>(
            `/api/mempool/lightning/nodes/${channel.remotePubkey}`
          );
          return response;
        } catch (error) {
          console.error(error);
          return undefined;
        }
      })
    );
    setNodes(nodes.filter((node) => !!node) as Node[]);
  }, [channels]);

  React.useEffect(() => {
    loadNodeStats();
  }, [loadNodeStats]);

  const lightningBalance = channels
    ?.map((channel) => channel.localBalance)
    .reduce((a, b) => a + b, 0);

  async function closeChannel(
    channelId: string,
    nodeId: string,
    isActive: boolean
  ) {
    try {
      if (!csrf) {
        throw new Error("csrf not loaded");
      }
      if (!isActive) {
        if (
          !confirm(
            `This channel is inactive. Some channels require up to 6 onchain confirmations before they are usable. If you really want to continue, click OK.`
          )
        ) {
          return;
        }
      }
      if (
        !confirm(
          `Are you sure you want to close the channel with ${nodes.find((node) => node.public_key === nodeId)?.alias ||
          "Unknown Node"
          }?\n\nNode ID: ${nodeId}\n\nChannel ID: ${channelId}`
        )
      ) {
        return;
      }

      console.log(`🎬 Closing channel with ${nodeId}`);

      const closeChannelResponse = await request<CloseChannelResponse>(
        `/api/peers/${nodeId}/channels/${channelId}`,
        {
          method: "DELETE",
          headers: {
            "X-CSRF-Token": csrf,
            "Content-Type": "application/json",
          },
        }
      );

      if (!closeChannelResponse) {
        throw new Error("Error closing channel");
      }

      await reloadChannels();

      alert(`🎉 Channel closed`);
    } catch (error) {
      console.error(error);
      alert("Something went wrong: " + error);
    }
  }

  async function resetRouter() {
    try {
      if (!csrf) {
        throw new Error("csrf not loaded");
      }

      await request("/api/reset-router", {
        method: "POST",
        headers: {
          "X-CSRF-Token": csrf,
          "Content-Type": "application/json",
        },
      });
      await reloadInfo();
      alert(`🎉 Router reset`);
    } catch (error) {
      console.error(error);
      alert("Something went wrong: " + error);
    }
  }

  async function stopNode() {
    try {
      if (!csrf) {
        throw new Error("csrf not loaded");
      }

      await request("/api/stop", {
        method: "POST",
        headers: {
          "X-CSRF-Token": csrf,
          "Content-Type": "application/json",
        },
      });
      await reloadInfo();
      alert(`🎉 Node stopped`);
    } catch (error) {
      console.error(error);
      alert("Something went wrong: " + error);
    }
  }

  return (
    <>
      <AppHeader
        title="Liquidity"
        description="Manage your lightning node liquidity."
        contentRight={
          <>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="default">
                  Advanced
                  <ChevronDown />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent className="w-56">
                <DropdownMenuGroup>
                  <DropdownMenuItem>
                    <div className="flex flex-row gap-10 items-center w-full">
                      <div className="whitespace-nowrap flex flex-row items-center gap-2">
                        Node
                      </div>
                      <div className="overflow-hidden text-ellipsis">
                        {/* TODO: replace with skeleton loader */}
                        {nodeConnectionInfo?.pubkey || "Loading..."}
                      </div>
                      {nodeConnectionInfo && (
                        <CopyIcon
                          className="shrink-0 w-4 h-4"
                          onClick={() => {
                            copyToClipboard(nodeConnectionInfo.pubkey);
                            toast({ title: "Copied to clipboard." });
                          }}
                        />
                      )}
                    </div>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
                <DropdownMenuGroup>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem>
                    <Link to="/channels/onchain/new-address">
                      Onchain Address
                    </Link>
                  </DropdownMenuItem>
                  {(info?.backendType === "LDK" ||
                    info?.backendType === "GREENLIGHT") &&
                    (balances?.onchain.spendable || 0) > ONCHAIN_DUST_SATS && (
                      <DropdownMenuItem
                        onClick={redeemOnchainFunds.redeemFunds}
                        disabled={redeemOnchainFunds.isLoading}
                      >
                        Redeem Onchain Funds
                        {redeemOnchainFunds.isLoading && <Loading />}
                      </DropdownMenuItem>
                    )}
                </DropdownMenuGroup>
                {info?.backendType === "LDK" && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuGroup>
                      <DropdownMenuLabel>Management</DropdownMenuLabel>
                      <DropdownMenuItem onClick={resetRouter}>
                        Reset Router
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={stopNode}>
                        Restart
                      </DropdownMenuItem>
                    </DropdownMenuGroup>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
            <Link to="/channels/new">
              <Button>Open a channel</Button>
            </Link>
          </>
        }
      ></AppHeader>
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Alby Hosted Balance
            </CardTitle>
            <Hotel className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {albyBalance?.sats} sats
            </div>

          </CardContent>
          <CardFooter className="flex justify-end">
            <Button variant="outline">Migrate</Button>
          </CardFooter>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Savings Balance
            </CardTitle>
            <Bitcoin className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {!balances && (
              <div>
                <div className="animate-pulse d-inline ">
                  <div className="h-2.5 bg-primary rounded-full w-12 my-2"></div>
                </div>
              </div>
            )}
            <div className="text-2xl font-bold">
              {balances && (
                <>
                  {new Intl.NumberFormat().format(balances.onchain.spendable)}{" "}
                  sats
                  {balances &&
                    balances.onchain.spendable !== balances.onchain.total && (
                      <p className="text-xs text-muted-foreground animate-pulse">
                        +
                        {new Intl.NumberFormat().format(
                          balances.onchain.total - balances.onchain.spendable
                        )}{" "}
                        sats incoming
                      </p>
                    )}
                </>
              )}
            </div>
          </CardContent>
          <CardFooter className="flex justify-end">
            <Link to="onchain/new-address">
              <Button variant="outline">Deposit</Button>
            </Link>
          </CardFooter>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Spending Balance
            </CardTitle>
            <ArrowUp className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            {!channels && (
              <div>
                <div className="animate-pulse d-inline ">
                  <div className="h-2.5 bg-primary rounded-full w-12 my-2"></div>
                </div>
              </div>
            )}
            {lightningBalance !== undefined && (
              <div className="text-2xl font-bold">
                {new Intl.NumberFormat(undefined, {}).format(
                  Math.floor(lightningBalance / 1000)
                )}{" "}
                sats
              </div>
            )}
          </CardContent>
          <CardFooter className="flex justify-end">
            <Button variant="outline">Top Up</Button>
          </CardFooter>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Receiving Capacity
            </CardTitle>
            <ArrowDown className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {balances && new Intl.NumberFormat().format(
                Math.floor(
                  balances.lightning.totalReceivable / 1000
                )
              )}{" "}
              sats
            </div>
          </CardContent>
          <CardFooter className="flex justify-end">
            <Button variant="outline">Increase</Button>
          </CardFooter>
        </Card>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className="w-[80px]">Status</TableHead>
            <TableHead>Node</TableHead>
            <TableHead className="w-[150px]">Capacity</TableHead>
            <TableHead className="w-[150px]">Inbound</TableHead>
            <TableHead className="w-[150px]">Outbound</TableHead>
            <TableHead className="w-[50px]"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {channels && channels.length > 0 && (
            <>
              {channels.map((channel) => {
                const node = nodes.find(
                  (n) => n.public_key === channel.remotePubkey
                );
                const alias = node?.alias || "Unknown";
                const capacity = channel.localBalance + channel.remoteBalance;

                return (
                  <TableRow key={channel.id}>
                    <TableCell>
                      {channel.active ? <Badge>Online</Badge> : <Badge>Offline</Badge>}{" "}
                    </TableCell>
                    <TableCell className="flex flex-row items-center">
                      <a
                        title={channel.remotePubkey}
                        href={`https://amboss.space/node/${channel.remotePubkey}`}
                        target="_blank"
                        rel="noopener noreferer"
                      >
                        <Button variant="link" className="p-0 mr-2">
                          {alias ||
                            channel.remotePubkey.substring(0, 5) + "..."}
                        </Button>
                      </a>
                      <Badge variant="outline">
                        {channel.public ? "Public" : "Private"}
                      </Badge>
                    </TableCell>
                    <TableCell>{formatAmount(capacity)} sats</TableCell>
                    <TableCell>{formatAmount(channel.localBalance)} sats</TableCell>
                    <TableCell>{formatAmount(channel.remoteBalance)} sats</TableCell>
                    <TableCell>
                      <DropdownMenu>
                        <DropdownMenuTrigger>
                          <Button
                            size="icon"
                            variant="ghost"
                          >
                            <MoreHorizontal className="h-4 w-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem className="flex flex-row items-center gap-2" onClick={() => closeChannel(
                            channel.id,
                            channel.remotePubkey,
                            channel.active
                          )}>
                            <Trash2 className="text-destructive" />
                            Close Channel
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </TableCell>
                  </TableRow>
                );
              })}
            </>
          )}
          {!channels && (
            <TableRow>
              <TableCell colSpan={6}>
                <Loading className="m-2" />
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </>
  );
}

const formatAmount = (amount: number, decimals = 1) => {
  amount /= 1000; //msat to sat
  let i = 0;
  for (i; amount >= 1000; i++) {
    amount /= 1000;
  }
  return amount.toFixed(i > 0 ? decimals : 0) + ["", "k", "M", "G"][i];
};
