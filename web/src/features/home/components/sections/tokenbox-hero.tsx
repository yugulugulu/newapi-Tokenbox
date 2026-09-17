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
import { ArrowRight, BookOpen, Gift, Sparkles } from "lucide-react";
import { useTranslation } from "react-i18next";

import { Button } from "@/components/ui/button";
import { getLobeIcon } from "@/lib/lobe-icon";

import { TokenBoxMascot } from "../tokenbox-mascot";

const MODELS = [
  { name: "OpenAI", icon: "OpenAI.Color" },
  { name: "Claude", icon: "Claude.Color" },
  { name: "Gemini", icon: "Gemini.Color" },
  { name: "DeepSeek", icon: "DeepSeek.Color" },
  { name: "Qwen", icon: "Qwen.Color" },
] as const;

type TokenBoxHeroProps = {
  docsUrl: string;
  isAuthenticated: boolean;
};

export function TokenBoxHero(props: TokenBoxHeroProps) {
  const { t } = useTranslation();
  const accountUrl = props.isAuthenticated ? "/dashboard" : "/sign-up";
  const accountLabel = props.isAuthenticated
    ? t("Go to Dashboard")
    : t("home.tokenbox.hero.newcomer");
  const externalDocs = props.docsUrl.startsWith("http");

  return (
    <section className="relative isolate min-h-[710px] overflow-hidden border-b pt-28 md:pt-32 lg:min-h-[760px]">
      <div
        aria-hidden
        className="dark:bg-background pointer-events-none absolute inset-0 -z-20 bg-[#f5faff]"
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 -z-10 opacity-50 dark:opacity-20"
        style={{
          backgroundImage:
            "linear-gradient(to right, color-mix(in oklch, var(--primary) 14%, transparent) 1px, transparent 1px), linear-gradient(to bottom, color-mix(in oklch, var(--primary) 14%, transparent) 1px, transparent 1px)",
          backgroundSize: "64px 64px",
          maskImage:
            "linear-gradient(to bottom, black 0%, rgba(0,0,0,.7) 60%, transparent 100%)",
        }}
      />

      <div className="mx-auto grid max-w-7xl items-center gap-8 px-5 pb-16 sm:px-8 lg:grid-cols-[minmax(0,0.9fr)_minmax(480px,1.1fr)] lg:px-10 lg:pb-20">
        <div className="relative z-10 max-w-2xl">
          <div className="landing-animate-fade-up border-primary/30 bg-background/80 text-primary mb-6 inline-flex items-center gap-2 rounded-full border px-3.5 py-2 text-sm font-semibold opacity-0 shadow-sm backdrop-blur-sm">
            <Sparkles className="size-4" aria-hidden="true" />
            {t("home.tokenbox.hero.badge")}
          </div>

          <h1 className="landing-animate-fade-up text-foreground text-5xl leading-[1.05] font-black opacity-0 sm:text-6xl lg:text-7xl">
            <span className="block">{t("home.tokenbox.hero.titleLine1")}</span>
            <span className="mt-2 block">
              {t("home.tokenbox.hero.titleLine2")}
            </span>
          </h1>
          <p className="landing-animate-fade-up text-muted-foreground mt-6 max-w-xl text-lg leading-8 opacity-0 sm:text-xl">
            {t("home.tokenbox.hero.description")}
          </p>

          <div className="landing-animate-fade-up mt-8 flex flex-wrap gap-3 opacity-0">
            <Button
              className="h-12 rounded-xl px-5 text-base shadow-[0_10px_24px_rgba(8,123,255,0.25)]"
              render={<Link to={accountUrl} />}
            >
              <Gift className="size-5" aria-hidden="true" />
              {accountLabel}
              <ArrowRight className="size-4" aria-hidden="true" />
            </Button>
            <Button
              variant="outline"
              className="bg-background/85 h-12 rounded-xl px-5 text-base backdrop-blur-sm"
              render={<Link to="/pricing" />}
            >
              {t("home.tokenbox.hero.pricing")}
            </Button>
            <Button
              variant="outline"
              className="bg-background/85 h-12 rounded-xl px-5 text-base backdrop-blur-sm"
              render={
                externalDocs ? (
                  <a href={props.docsUrl} target="_blank" rel="noreferrer" />
                ) : (
                  <Link to={props.docsUrl} />
                )
              }
            >
              <BookOpen className="size-4" aria-hidden="true" />
              {t("home.tokenbox.hero.docs")}
            </Button>
          </div>

          <div className="landing-animate-fade-up mt-10 opacity-0">
            <p className="text-muted-foreground mb-3 text-sm font-medium">
              {t("home.tokenbox.hero.models")}
            </p>
            <div className="flex flex-wrap gap-2">
              {MODELS.map((model) => (
                <div
                  key={model.name}
                  aria-label={model.name}
                  className="border-border/70 bg-background/85 dark:bg-card/90 hover:border-primary/50 hover:shadow-primary/10 group flex h-10 items-center gap-2 rounded-xl border px-3.5 text-sm font-semibold shadow-sm backdrop-blur-sm transition duration-200 hover:-translate-y-1 hover:shadow-lg"
                >
                  <span className="bg-muted group-hover:bg-background flex size-6 items-center justify-center rounded-lg transition-colors">
                    {getLobeIcon(model.icon, 19)}
                  </span>
                  {model.name}
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className="relative mx-auto w-full max-w-[690px] lg:mt-0">
          <div
            aria-hidden
            className="absolute inset-x-[12%] bottom-[8%] h-[65%] rounded-[50%] bg-sky-200/55 blur-3xl dark:bg-white/6"
          />
          <div className="tokenbox-mascot-float relative">
            <TokenBoxMascot />
          </div>
        </div>
      </div>
    </section>
  );
}
