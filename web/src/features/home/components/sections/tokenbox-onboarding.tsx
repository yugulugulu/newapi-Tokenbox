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
import {
  ArrowRight,
  CircleUserRound,
  KeyRound,
  TerminalSquare,
} from "lucide-react";
import { useTranslation } from "react-i18next";

import { AnimateInView } from "@/components/animate-in-view";
import { Button } from "@/components/ui/button";

export function TokenBoxOnboarding() {
  const { t } = useTranslation();
  const steps = [
    {
      number: "01",
      icon: CircleUserRound,
      title: t("home.tokenbox.onboarding.step1Title"),
      description: t("home.tokenbox.onboarding.step1Description"),
      color: "bg-cyan-500",
    },
    {
      number: "02",
      icon: KeyRound,
      title: t("home.tokenbox.onboarding.step2Title"),
      description: t("home.tokenbox.onboarding.step2Description"),
      color: "bg-rose-500",
    },
    {
      number: "03",
      icon: TerminalSquare,
      title: t("home.tokenbox.onboarding.step3Title"),
      description: t("home.tokenbox.onboarding.step3Description"),
      color: "bg-amber-400",
    },
  ];

  return (
    <section className="bg-background px-5 py-20 sm:px-8 lg:px-10">
      <div className="mx-auto max-w-7xl">
        <AnimateInView className="mb-9 flex flex-col justify-between gap-5 sm:flex-row sm:items-end">
          <div>
            <h2 className="text-3xl font-black sm:text-4xl">
              {t("home.tokenbox.onboarding.title")}
            </h2>
            <p className="text-muted-foreground mt-3 text-base sm:text-lg">
              {t("home.tokenbox.onboarding.description")}
            </p>
          </div>
          <Button
            className="h-11 self-start rounded-xl px-4 sm:self-auto"
            render={<Link to="/keys" />}
          >
            <KeyRound className="size-4" aria-hidden="true" />
            {t("home.tokenbox.onboarding.createKey")}
            <ArrowRight className="size-4" aria-hidden="true" />
          </Button>
        </AnimateInView>

        <AnimateInView className="border-border bg-card grid overflow-hidden rounded-2xl border shadow-[0_18px_50px_rgba(32,100,170,0.09)] lg:grid-cols-[300px_1fr] dark:shadow-[0_18px_50px_rgba(0,0,0,0.22)]">
          <div className="relative isolate flex min-h-60 flex-col justify-end overflow-hidden bg-[#087bff] p-7 text-white">
            <div
              aria-hidden
              className="absolute -top-10 -right-10 -z-10 size-44 rotate-12 rounded-3xl border-[22px] border-white/12"
            />
            <div
              aria-hidden
              className="absolute top-16 left-8 -z-10 size-16 -rotate-12 rounded-xl border-[10px] border-white/12"
            />
            <div className="mb-auto flex size-14 items-center justify-center rounded-2xl bg-white/18 text-2xl font-black shadow-inner">
              +
            </div>
            <p className="text-3xl font-black">
              {t("home.tokenbox.onboarding.benefit")}
            </p>
            <p className="mt-2 max-w-56 text-sm leading-6 text-blue-50">
              {t("home.tokenbox.onboarding.benefitDescription")}
            </p>
          </div>

          <ol className="relative grid gap-0 lg:grid-cols-3">
            {steps.map((step, index) => {
              const Icon = step.icon;
              return (
                <li
                  key={step.number}
                  className="relative flex min-h-60 flex-col items-center justify-center px-6 py-8 text-center"
                >
                  {index < steps.length - 1 && (
                    <div
                      aria-hidden
                      className="border-primary/50 absolute top-1/2 right-0 hidden w-1/4 translate-x-1/2 border-t-2 border-dashed lg:block"
                    />
                  )}
                  <span
                    className={`${step.color} absolute top-6 flex size-9 items-center justify-center rounded-full text-sm font-black text-white shadow-lg`}
                  >
                    {step.number}
                  </span>
                  <div className="bg-muted text-primary mt-7 flex size-20 items-center justify-center rounded-2xl border shadow-sm">
                    <Icon className="size-10" aria-hidden="true" />
                  </div>
                  <h3 className="mt-5 text-lg font-extrabold">{step.title}</h3>
                  <p className="text-muted-foreground mt-1 text-sm">
                    {step.description}
                  </p>
                </li>
              );
            })}
          </ol>
        </AnimateInView>
      </div>
    </section>
  );
}
