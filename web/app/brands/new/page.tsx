// Where a brand owner starts: type the brand, get a dashboard.
//
// The page is a heading and a form. It fetches nothing, because there is nothing
// to read before a brand exists.

import { NewBrandForm } from "@/components/NewBrandForm";

export const metadata = {
  title: "Add a brand | BrandPulse",
};

export default function NewBrandPage() {
  return (
    <section className="section">
      <div className="section-head">
        <h2>Add a brand</h2>
        <p>
          bp-onboarder reads the website and works out the keywords, products and competitors to
          watch. What you type below is kept as well, and it is what the brand is monitored on if
          that agent cannot be reached.
        </p>
      </div>
      <div className="card">
        <NewBrandForm />
      </div>
    </section>
  );
}
