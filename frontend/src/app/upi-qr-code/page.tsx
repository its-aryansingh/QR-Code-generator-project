import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free UPI QR Code Generator | QRit',
  description: 'Create a UPI QR code to accept payments instantly. Perfect for Indian businesses, street vendors, and freelancers. Free and fast.',
  keywords: ['upi qr code generator', 'bhim upi qr', 'phonepe qr generator', 'gpay qr code maker'],
  alternates: {
    canonical: 'https://qrit.com/upi-qr-code'
  }
};

export default function UpiQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "UPI QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "FinanceApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "INR"
    }
  };

  const howToSteps = [
    { title: "Enter UPI ID", desc: "Type your Virtual Payment Address (VPA) correctly (e.g., name@bankname)." },
    { title: "Set Details", desc: "Add your Payee Name and optionally specify a fixed amount or transaction note." },
    { title: "Generate & Display", desc: "Download the QR code, print it, and display it at your store to accept payments." }
  ];

  const useCases = [
    { title: "Street Vendors & Shops", desc: "Accept digital payments seamlessly without needing a PoS machine or physical card reader." },
    { title: "Freelancers", desc: "Add a UPI QR code to your invoices so clients can pay you instantly with a quick scan." },
    { title: "Donations & NGOs", desc: "Collect funds easily at events or charity drives by displaying a fixed-amount UPI QR." },
    { title: "Service Providers", desc: "Plumbers, electricians, and home tutors can let customers pay digitally on the spot." }
  ];

  const faqs = [
    { question: "Which apps can scan this UPI QR code?", answer: "Any UPI-enabled app in India can scan it, including Google Pay (GPay), PhonePe, Paytm, BHIM, Amazon Pay, and most banking apps." },
    { question: "Do I have to enter an amount?", answer: "No. If you leave the amount blank, the customer can enter the amount themselves after scanning the code." },
    { question: "Is my payment safe?", answer: "Yes, the QR code merely contains your UPI ID routing instructions. The actual payment is processed securely by the user's UPI app and bank." },
    { question: "Are there any fees?", answer: "No, generating a UPI QR code with QRit is completely free, and standard UPI transactions generally do not have fees for basic peer-to-peer transfers." },
    { question: "Why is the scan failing?", answer: "Double-check your UPI ID spelling. Ensure the generated code is printed clearly without distortion." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="upi"
        heroH1="Free UPI QR Code Generator"
        heroSubtitle="Accept payments seamlessly. Generate a standard UPI QR code compatible with Google Pay, PhonePe, Paytm, and BHIM."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
