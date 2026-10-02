import type { Metadata } from 'next';
import { QRTypePage } from '@/components/seo/QRTypePage';

export const metadata: Metadata = {
  title: 'Free vCard QR Code Generator | QRit',
  description: 'Create a vCard QR code to easily share your contact details. Add name, phone, email, and company info to a scannable QR code.',
  keywords: ['vcard qr code', 'contact qr code generator', 'business card qr', 'digital business card'],
  alternates: {
    canonical: 'https://qrit.com/vcard-qr-code'
  }
};

export default function VCardQRPage() {
  const jsonLd = {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "vCard QR Code Generator - QRit",
    "operatingSystem": "Web",
    "applicationCategory": "BusinessApplication",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "USD"
    }
  };

  const howToSteps = [
    { title: "Enter Your Details", desc: "Fill in your name, phone number, email, and company details." },
    { title: "Review Information", desc: "Check that all your contact information is correct and formatted properly." },
    { title: "Download & Print", desc: "Download your QR code to print on your business cards or digital profiles." }
  ];

  const useCases = [
    { title: "Business Cards", desc: "Modernize your paper business cards by adding a QR code that saves your contact info directly to phones." },
    { title: "Conferences & Networking", desc: "Share your details instantly without running out of printed business cards." },
    { title: "Real Estate Agents", desc: "Allow potential buyers to save your contact information from signboards and flyers." },
    { title: "Email Signatures", desc: "Include a vCard QR code in your digital signature for easy saving." }
  ];

  const faqs = [
    { question: "What is a vCard QR code?", answer: "A vCard QR code contains all your contact information. When scanned, it prompts the user's phone to create a new contact with your details pre-filled." },
    { question: "Which fields are required?", answer: "Only your name is strictly required, but adding a phone number or email makes the vCard useful." },
    { question: "Does this work on iPhones and Android?", answer: "Yes, both iOS and Android natively support scanning vCard QR codes via their default camera apps." },
    { question: "Can I update my vCard later?", answer: "Standard vCard QR codes are static, meaning you cannot change the details once printed. To update details later, you need to use a Dynamic QR code." },
    { question: "Is my personal data safe?", answer: "Yes. The QR code simply encodes the text you enter. We do not store your contact data on our servers for static QR codes." }
  ];

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
      />
      <QRTypePage
        type="vcard"
        heroH1="Free vCard QR Code Generator"
        heroSubtitle="Turn your contact information into a scannable QR code. The easiest way to share your details and network efficiently."
        howToSteps={howToSteps}
        useCases={useCases}
        faqs={faqs}
      />
    </>
  );
}
