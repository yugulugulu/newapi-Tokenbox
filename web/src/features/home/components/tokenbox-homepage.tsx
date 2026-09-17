/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from "@tanstack/react-router";
import { ArrowRight } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { useStatus } from "@/hooks/use-status";

import { TokenBoxCommunity } from "./sections/tokenbox-community";
import { TokenBoxHero } from "./sections/tokenbox-hero";
import { TokenBoxOnboarding } from "./sections/tokenbox-onboarding";

type TokenBoxHomepageProps = {
  isAuthenticated: boolean;
};

export function TokenBoxHomepage(props: TokenBoxHomepageProps) {
  const { t } = useTranslation();
  const { status } = useStatus();
  const docsUrl =
    (status?.docs_link as string | undefined) || "https://docs.newapi.pro";

  return (
    <main>
      <TokenBoxHero docsUrl={docsUrl} isAuthenticated={props.isAuthenticated} />
      <TokenBoxOnboarding />
      <TokenBoxCommunity />

      <section className="border-border dark:bg-card border-y bg-[#f6faff] px-5 py-12 sm:px-8">
        <div className="mx-auto flex max-w-7xl flex-col justify-between gap-6 sm:flex-row sm:items-center">
          <div>
            <h2 className="text-2xl font-black">
              {t("home.tokenbox.final.title")}
            </h2>
            <p className="text-muted-foreground mt-2">
              {t("home.tokenbox.final.description")}
            </p>
          </div>
          <Button
            className="h-12 self-start rounded-xl px-5 sm:self-auto dark:bg-white dark:text-black dark:hover:bg-white/85"
            render={<Link to="/pricing" />}
          >
            {t("home.tokenbox.final.pricing")}
            <ArrowRight className="size-4" aria-hidden="true" />
          </Button>
        </div>
      </section>
    </main>
  );
}
