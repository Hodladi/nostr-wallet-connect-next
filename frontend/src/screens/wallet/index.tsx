import { ExternalLinkIcon } from "lucide-react";
import { Link } from "react-router-dom";
import AlbyHead from "src/assets/images/alby-head.svg";
import AppHeader from "src/components/AppHeader";
import BreezRedeem from "src/components/BreezRedeem";
import ExternalLink from "src/components/ExternalLink";
import Loading from "src/components/Loading";
import { Button } from "src/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "src/components/ui/card";
import { Checkbox } from "src/components/ui/checkbox";
import { useBalances } from "src/hooks/useBalances";
import { useInfo } from "src/hooks/useInfo";

function Wallet() {
  const { data: info } = useInfo();
  const { data: balances } = useBalances();

  /* eslint-disable  @typescript-eslint/no-explicit-any */
  const extensionInstalled = (window as any).alby !== undefined;

  if (!info || !balances) {
    return <Loading />;
  }

  return (
    <>
      <AppHeader title="Wallet" description="Send and receive transactions" />

      <div className="flex flex-col lg:flex-row justify-between items-start lg:items-end gap-5">
        <div className="text-5xl font-semibold">
          {new Intl.NumberFormat().format(
            Math.floor(balances.lightning.totalSpendable / 1000)
          )}{" "}
          sats
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <ExternalLink to="https://www.getalby.com/dashboard">
          <Card>
            <CardHeader>
              <div className="flex flex-row items-center">
                <img
                  src={AlbyHead}
                  className="w-12 h-12 rounded-xl p-1 border"
                />
                <div>
                  <CardTitle>
                    <div className="flex-1 leading-5 font-semibold text-xl whitespace-nowrap text-ellipsis overflow-hidden ml-4">
                      Alby Web
                    </div>
                  </CardTitle>
                  <CardDescription className="ml-4">
                    Install Alby Web on your phone and use your Hub on the go.
                  </CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="text-right">
              <Button variant="outline">
                Open Alby Web
                <ExternalLinkIcon className="w-4 h-4 ml-2" />
              </Button>
            </CardContent>
          </Card>
        </ExternalLink>
        {!extensionInstalled && (
          <Link to={`https://www.getalby.com`} target="_blank">
            <Card>
              <CardHeader>
                <div className="flex flex-row items-center">
                  <img
                    src={AlbyHead}
                    className="w-12 h-12 rounded-xl p-1 border bg-[#FFDF6F]"
                  />
                  <div>
                    <CardTitle>
                      <div className="flex-1 leading-5 font-semibold text-xl whitespace-nowrap text-ellipsis overflow-hidden ml-4">
                        Alby Browser Extension
                      </div>
                    </CardTitle>
                    <CardDescription className="ml-4">
                      Seamless bitcoin payments in your favourite internet
                      browser.
                    </CardDescription>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="text-right">
                <Button variant="outline">
                  Install Alby Extension
                  <ExternalLinkIcon className="w-4 h-4 ml-2" />
                </Button>
              </CardContent>
            </Card>
          </Link>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Get started with your Alby Hub</CardTitle>
          <CardDescription>Some first steps to get you started</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <CheckboxItem
            id="open-channel"
            title="Open your first channel"
            description="alkdsfjla ksdjflaksjdfljaksdf "
            checked={true}
          />
          <CheckboxItem
            id="link-alby"
            title="Link your Alby Account"
            description="description"
            checked={true}
          />
          <CheckboxItem
            id="create-app"
            title="Create your first app connection"
            description="description"
            checked={false}
          />
          <CheckboxItem
            id="backup-keys"
            title="Backup your keys"
            description="description"
            checked={false}
          />
          <CheckboxItem
            id="make-payment"
            title="Make first payment"
            description="description"
            checked={false}
          />
          <CheckboxItem
            id="help-friend"
            title="Help a friend to get on lightning"
            description="description"
            checked={false}
          />
        </CardContent>
      </Card>

      <BreezRedeem />
    </>
  );
}

function CheckboxItem({
  id,
  title,
  checked,
  description,
}: {
  id: string;
  title: string;
  checked: boolean;
  description: string;
}) {
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <Checkbox id={id} checked={checked} />
        <label
          htmlFor={id}
          className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
        >
          {title}
        </label>
      </div>
      {!checked && (
        <div className="text-muted-foreground text-sm ml-6">{description}</div>
      )}
    </div>
  );
}

export default Wallet;
